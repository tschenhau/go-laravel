package celeritas

import (
	"context"
	"fmt"
	"github.com/CloudyKit/jet/v6"
	"github.com/alexedwards/scs/v2"
	"github.com/dgraph-io/badger/v3"
	"github.com/go-chi/chi/v5"
	"github.com/gomodule/redigo/redis"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/tsawler/celeritas/cache"
	"github.com/tsawler/celeritas/mailer"
	"github.com/tsawler/celeritas/render"
	"github.com/tsawler/celeritas/session"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const maxWorkerPool = 20
const version = "1.0.0"

var redisPool *redis.Pool
var badgerConn *badger.DB
var myRedisCache *cache.RedisCache
var myBadgerCache *cache.BadgerCache
var errorLogFile *os.File
var infoLogFile *os.File

// Celeritas is the overall type for the application
type Celeritas struct {
	AppName       string
	Debug         bool
	ErrorLog      *log.Logger
	InfoLog       *log.Logger
	Version       string
	RootPath      string
	Session       *scs.SessionManager
	DB            Database
	Cache         cache.Cache
	Render        *render.Render
	Routes        *chi.Mux
	EncryptionKey []byte
	JetViews      *jet.Set
	config        config
	Server        Server
	Mail          mailer.Mail
	Scheduler     *cron.Cron
}

// config holds all application config values
type config struct {
	port          string
	renderer      string
	database      databaseConfig
	cookie        cookieConfig
	redis         redisConfig
	sessionType   string
	encryptionKey string
}

// New reads the .env file, creates our application config, populates the Celeritas type with settings
// based on .env values, and creates necessary folders and files if they don't exist, and generally
// sets up the things we need to make this application work.
func (c *Celeritas) New(rootPath string) error {

	pid := os.Getpid()
	log.Println("Current PID:", pid)

	// create necessary folders if they don't exist
	pathConfig := initPaths{
		rootPath:    rootPath,
		folderNames: []string{"handlers", "migrations", "views", "data", "public", "mail", "tmp", "logs", "middleware"},
	}

	err := c.Init(pathConfig)
	if err != nil {
		return err
	}

	// create .env if it does not exist
	err = c.checkDotEnv(rootPath)
	if err != nil {
		return err
	}

	// read .env
	err = godotenv.Load(rootPath + "/.env")
	if err != nil {
		return err
	}

	// check if we are in debug mode
	debug, _ := strconv.ParseBool(os.Getenv("DEBUG"))

	// create simple loggers
	infoLog, errorLog := c.startLoggers(debug)

	// connect to database
	if os.Getenv("DATABASE_TYPE") != "" {
		os.Setenv("UPPER_DB_LOG", "ERROR")
		db, err := c.OpenDB(os.Getenv("DATABASE_TYPE"), c.BuildDSN())
		if err != nil {
			errorLog.Println(err)
			os.Exit(1)
		}
		c.DB = Database{
			DatabaseType: os.Getenv("DATABASE_TYPE"),
			Pool:         db,
		}
	}

	// create the scheduler and add it to Celeritas
	scheduler := cron.New()
	c.Scheduler = scheduler

	// set up the cache
	// if we are using redis at all, create the connection pool (stored as package level variable)
	if os.Getenv("CACHE") == "redis" || os.Getenv("SESSION_TYPE") == "redis" {
		myRedisCache = c.createClientRedisCache()
		c.Cache = myRedisCache
	}

	// if we are using badger at all, create the badger connection (stored as a package level variable)
	if os.Getenv("CACHE") == "badger" {
		myBadgerCache = c.createClientBadgerCache()
		c.Cache = myBadgerCache

		// we need to run periodic garbage collection on badger, so run that once a day
		_, err = c.Scheduler.AddFunc("@daily", func() {
			_ = myBadgerCache.Conn.RunValueLogGC(0.7)
		})
		if err != nil {
			return err
		}
	}

	// populate the rest of the Celeritas values
	c.RootPath = rootPath
	c.AppName = os.Getenv("APP_NAME")
	c.Debug = debug
	c.Mail = c.createMailer()
	c.InfoLog = infoLog
	c.ErrorLog = errorLog
	c.Version = version
	c.Routes = c.routes().(*chi.Mux)
	c.config = config{
		port:     os.Getenv("PORT"),
		renderer: os.Getenv("RENDERER"),
		database: databaseConfig{
			database: os.Getenv("DATABASE_TYPE"),
			dsn:      c.BuildDSN(),
		},
		cookie: cookieConfig{
			name:     os.Getenv("COOKIE_NAME"),
			lifetime: os.Getenv("COOKIE_LIFETIME"),
			persist:  os.Getenv("COOKIE_PERSISTS"),
			secure:   os.Getenv("COOKIE_SECURE"),
			domain:   os.Getenv("COOKIE_DOMAIN"), // add this
		},
		sessionType: os.Getenv("SESSION_TYPE"),
		redis: redisConfig{
			host:     os.Getenv("REDIS_HOST"),
			password: os.Getenv("REDIS_PASSWORD"),
			prefix:   os.Getenv("REDIS_PREFIX"),
		},
		encryptionKey: os.Getenv("KEY"),
	}
	secure := true
	if strings.ToLower(os.Getenv("SECURE")) == "false" {
		secure = false
	}
	c.Server = Server{
		ServerName: os.Getenv("SERVER_NAME"),
		Port:       os.Getenv("PORT"),
		Secure:     secure,
	}

	sess := session.Session{
		CookieLifetime: c.config.cookie.lifetime,
		CookiePersist:  c.config.cookie.persist,
		CookieName:     c.config.cookie.name,
		CookieDomain:   c.config.cookie.domain,
		SessionType:    c.config.sessionType,
		DBPool:         c.DB.Pool,
	}

	// only add fields that have non-nil values
	switch c.config.sessionType {
	case "redis":
		sess.RedisPool = myRedisCache.Conn
	case "badger":
		sess.BadgerConn = myBadgerCache.Conn
	case "mysql", "postgres", "mariadb":
		sess.DBPool = c.DB.Pool
	}

	c.Session = sess.InitSession()

	c.EncryptionKey = []byte(os.Getenv("KEY"))

	// we need the *jet.Set type in order to render jet templates,
	// so let's add it to the Celeritas type
	if c.Debug {
		var views = jet.NewSet(
			jet.NewOSFileSystemLoader(fmt.Sprintf("%s/views", rootPath)),
			jet.InDevelopmentMode(),
		)
		c.JetViews = views
	} else {
		var views = jet.NewSet(
			jet.NewOSFileSystemLoader(fmt.Sprintf("%s/views", rootPath)),
		)
		c.JetViews = views
	}

	c.createRenderer()

	// start listening on the mail channel
	go c.Mail.ListenForMail()

	return nil
}

// startLoggers creates loggers. If running in debug mode, logs are written to stdout;
// otherwise, they are written to the logs folder
func (c *Celeritas) startLoggers(debug bool) (*log.Logger, *log.Logger) {
	var infoLog *log.Logger
	var errorLog *log.Logger

	if debug {
		infoLog = log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
		errorLog = log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		errorFile, err := os.OpenFile(c.RootPath+"/logs/error_log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening error log file: %v", err)
		}
		errorLog = log.New(errorFile, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
		errorLogFile = errorFile

		infoFile, err := os.OpenFile(c.RootPath+"/logs/info_log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening info log file: %v", err)
		}
		infoLog = log.New(infoFile, "INFO\t", log.Ldate|log.Ltime)
		infoLogFile = infoFile
	}
	return infoLog, errorLog
}

// checkDotEnv creates an empty .env file in application folder
// if it does not exist
func (c *Celeritas) checkDotEnv(rootPath string) error {
	err := c.CreateFileIfNotExists(fmt.Sprintf("%s/.env", rootPath))
	if err != nil {
		return err
	}
	return nil
}

// createClientRedisCache creates redis cache
func (c *Celeritas) createClientRedisCache() *cache.RedisCache {
	cacheClient := cache.RedisCache{
		Conn:   c.createRedisPool(),
		Prefix: c.config.redis.prefix,
	}
	return &cacheClient
}

// createClientBadgerCache creates badger cache
func (c *Celeritas) createClientBadgerCache() *cache.BadgerCache {
	cacheClient := cache.BadgerCache{
		Conn:   c.createBadgerConn(),
		Prefix: os.Getenv("APP_NAME"),
	}
	return &cacheClient
}

func (c *Celeritas) createBadgerConn() *badger.DB {
	db, err := badger.Open(badger.DefaultOptions(c.RootPath + "/tmp/badger"))
	if err != nil {
		return nil
	}
	return db
}

// createRenderer creates and initializes a new renderer
func (c *Celeritas) createRenderer() {
	myRenderer := render.Render{
		Renderer:   c.config.renderer,
		Cache:      c.Cache,
		RootPath:   c.RootPath,
		JetViews:   c.JetViews,
		ServerName: c.Server.ServerName,
		Port:       c.Server.Port,
		Secure:     c.Server.Secure,
		Session:    c.Session,
	}
	c.Render = &myRenderer
}

// createRedisPool creates a redis connection pool
func (c *Celeritas) createRedisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     50,
		MaxActive:   10000,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp",
				c.config.redis.host,
				redis.DialPassword(c.config.redis.password))
		},

		TestOnBorrow: func(conn redis.Conn, t time.Time) error {
			_, err := conn.Do("PING")
			return err
		},
	}
}

// createMailer sets the necessary values for our mailer package
// and returns a mailer.Mail
func (c *Celeritas) createMailer() mailer.Mail {
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	s := mailer.Mail{
		Domain:      os.Getenv("MAIL_DOMAIN"),
		Templates:   c.RootPath + "/mail",
		Host:        os.Getenv("SMTP_HOST"),
		Port:        port,
		Username:    os.Getenv("SMTP_USERNAME"),
		Password:    os.Getenv("SMTP_PASSWORD"),
		Encryption:  os.Getenv("SMTP_ENCRYPTION"),
		FromName:    os.Getenv("FROM_NAME"),
		FromAddress: os.Getenv("FROM_ADDRESS"),
		Jobs:        make(chan mailer.Message, maxWorkerPool),
		Results:     make(chan mailer.Result, maxWorkerPool),
		API:         os.Getenv("MAILER_API"),
		APIKey:      os.Getenv("MAILER_KEY"),
		APIUrl:      os.Getenv("MAILER_URL"),
	}

	return s
}

// ListenAndServe starts the web server
func (c *Celeritas) ListenAndServe() error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", os.Getenv("PORT")),
		ErrorLog:          c.ErrorLog,
		Handler:           c.Routes,
		IdleTimeout:       30 * time.Second,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      600 * time.Second,
	}

	shutdownError := make(chan error)

	// start a goroutine in the background that listens for signals, so we
	// can run cleanup tasks before exiting (interrupt or terminate signals)
	go func() {

		// we create a buffered channel (size 1) because signal.Notify() does not wait for a receiver to be
		//available when sending a signal to the quit channel.
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		c.InfoLog.Println("received signal:", s)

		// give processes 5 seconds to complete
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			c.ErrorLog.Println("Error shutting down:", err)
			shutdownError <- err
		}

		// cleanup tasks go here...
		if c.DB.Pool != nil {
			defer c.DB.Pool.Close()
		}
		if redisPool != nil {
			defer redisPool.Close()
		}

		if badgerConn != nil {
			defer badgerConn.Close()
		}

		if !c.Debug {
			defer errorLogFile.Close()
			defer infoLogFile.Close()
		}

		defer close(c.Mail.Jobs)

		// add waitgroup here

		shutdownError <- nil
	}()

	c.InfoLog.Printf("Listening on port %s....", os.Getenv("PORT"))
	err := srv.ListenAndServe()

	err = <-shutdownError
	if err != nil {
		return err
	}

	return nil
}

// BuildDSN builds the connection string appropriate for our database based on .env values
func (c *Celeritas) BuildDSN() string {
	var dsn string

	switch os.Getenv("DATABASE_TYPE") {
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s timezone=UTC connect_timeout=5",
			os.Getenv("DATABASE_HOST"),
			os.Getenv("DATABASE_PORT"),
			os.Getenv("DATABASE_USER"),
			os.Getenv("DATABASE_NAME"),
			os.Getenv("DATABASE_SSL_MODE"))
		if os.Getenv("DATABASE_PASS") != "" {
			dsn = fmt.Sprintf("%s password=%s", dsn, os.Getenv("DATABASE_PASS"))
		}
	case "mysql", "mariadb":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?collation=utf8_unicode_ci&timeout=5s&parseTime=true&tls=%s&readTimeout=5s",
			os.Getenv("DATABASE_USER"),
			os.Getenv("DATABASE_PASS"),
			os.Getenv("DATABASE_HOST"),
			os.Getenv("DATABASE_PORT"),
			os.Getenv("DATABASE_NAME"),
			os.Getenv("DATABASE_SSL_MODE"))
	}
	return dsn
}
