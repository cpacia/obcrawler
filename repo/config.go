package repo

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultConfigFilename = "obcrawler.conf"
	defaultLogFilename    = "crawler.log"
)

var log = logging.MustGetLogger("REPO")

var (
	DefaultHomeDir    = AppDataDir("obcrawler", false)
	defaultConfigFile = filepath.Join(DefaultHomeDir, defaultConfigFilename)

	fileLogFormat   = logging.MustStringFormatter(`%{time:2006-01-02 T15:04:05.000} [%{level}] [%{module}] %{message}`)
	stdoutLogFormat = logging.MustStringFormatter(`%{color:reset}%{color}%{time:15:04:05} [%{level}] [%{module}] %{message}`)
	LogLevelMap     = map[string]logging.Level{
		"debug":    logging.DEBUG,
		"info":     logging.INFO,
		"notice":   logging.NOTICE,
		"warning":  logging.WARNING,
		"error":    logging.ERROR,
		"critical": logging.CRITICAL,
	}

	DefaultMainnetBootstrapAddrs = []string{
		"/ip4/167.172.126.176/tcp/4001/p2p/12D3KooWPZ3xBNRGx4fhRbfYAcXUhcZhTZ2LCkJ74kJXGfz9TVLT",
	}

	defaultTestnetBootstrapAddrs = []string{
		"",
	}
)

// Config defines the configuration options for OpenBazaar.
//
// See loadConfig for details on the configuration load process.
type Config struct {
	NumNodes           uint          `short:"n" long:"nodes" description:"Number of IPFS nodes to spin up." default:"10"`
	NumWorkers         uint          `short:"w" long:"workers" description:"Number of workers to use when crawling nodes" default:"12"`
	PubsubNodes        uint          `short:"p" long:"pubsubnodes" description:"Number of pubsub nodes to listen on." default:"3"`
	ShowVersion        bool          `short:"v" long:"version" description:"Display version information and exit"`
	ConfigFile         string        `short:"C" long:"configfile" description:"Path to configuration file"`
	DataDir            string        `short:"d" long:"datadir" description:"Directory to store data"`
	CrawlInterval      time.Duration `long:"crawlinterval" description:"The amount of time to wait between network crawls" default:"1m"`
	LogDir             string        `long:"logdir" description:"Directory to log output."`
	LogLevel           string        `short:"l" long:"loglevel" description:"Set the logging level [debug, info, notice, warning, error, critical]." default:"info"`
	BoostrapAddrs      []string      `long:"bootstrapaddr" description:"Override the default bootstrap addresses with the provided values"`
	Testnet            bool          `short:"t" long:"testnet" description:"Use the test network"`
	DisableNATPortMap  bool          `long:"noupnp" description:"Disable use of upnp"`
	IPNSQuorum         uint          `long:"ipnsquorum" description:"The size of the IPNS quorum to use. Smaller is faster but less up-to-date." default:"2"`
	UserAgentComment   string        `long:"uacomment" description:"Comment to add to the user agent"`
	DisableDataCaching bool          `long:"disabledatacaching" description:"By default the crawler will download, cache, and seed images and ratings. This functionality can be disabled with this flag."`
	DisableFilePinning bool          `long:"diablefilepinning" description:"By default the crawler will pin all files it downloads until the file is replaced by another one."`

	RPCCert       string `long:"rpccert" description:"A path to the SSL certificate to use with gRPC"`
	RPCKey        string `long:"rpckey" description:"A path to the SSL key to use with gRPC"`
	GrpcListener  string `long:"grpclisten" description:"Add an interface/port to listen for experimental gRPC connections (default port:5001)"`
	GrpcAuthToken string `long:"grpcauthtoken" description:"Set a token here if you want to enable client authentication with gRPC"`

	DBDialect string `long:"dbdialect" description:"The type of database to use [sqlite3, mysql, postgress]" default:"sqlite3"`
	DBHost    string `long:"dbhost" description:"The host:post location of the database."`
	DBUser    string `long:"dbuser" description:"The database username"`
	DBPass    string `long:"dbpass" description:"The database password"`
}

// LoadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
// 	1) Start with a default config with sane settings
// 	2) Pre-parse the command line to check for an alternative config file
// 	3) Load configuration file overwriting defaults with any specified options
// 	4) Parse CLI options and overwrite/add any specified options
//
// The above results in OpenBazaar functioning properly without any config settings
// while still allowing the user to override settings with config files and
// command line options.  Command line options always take precedence.
func LoadConfig() (*Config, error) {
	// Default config.
	cfg := Config{
		DataDir:    DefaultHomeDir,
		ConfigFile: defaultConfigFile,
	}

	// Pre-parse the command line options to see if an alternative config
	// file or the version flag was specified.  Any errors aside from the
	// help message error can be ignored here since they will be caught by
	// the final parse below.
	preCfg := cfg
	preParser := flags.NewParser(&cfg, flags.HelpFlag)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			return nil, err
		}
	}
	if cfg.DataDir != "" {
		preCfg.ConfigFile = filepath.Join(cfg.DataDir, defaultConfigFilename)
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show usage", appName)
	if preCfg.ShowVersion {
		fmt.Println(appName, "version", VersionString())
		os.Exit(0)
	}

	// Load additional config from file.
	var configFileError error
	parser := flags.NewParser(&cfg, flags.Default)
	if _, err := os.Stat(preCfg.ConfigFile); os.IsNotExist(err) {
		err := createDefaultConfigFile(preCfg.ConfigFile, cfg.Testnet)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating a "+
				"default config file: %v\n", err)
		}
	}

	err = flags.NewIniParser(parser).ParseFile(preCfg.ConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing config "+
				"file: %v\n", err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, err
		}
		configFileError = err
	}

	if cfg.NumNodes == 0 {
		return nil, errors.New("IPFS nodes must not be zero")
	}

	if cfg.NumWorkers == 0 {
		return nil, errors.New("workers must not be zero")
	}

	if cfg.PubsubNodes == 0 {
		return nil, errors.New("pubsub nodes must not be zero")
	}

	if cfg.PubsubNodes > cfg.NumNodes {
		return nil, errors.New("pubsub nodes must not exceeds the number of IPFS nodes")
	}

	_, ok := LogLevelMap[strings.ToLower(cfg.LogLevel)]
	if !ok {
		return nil, errors.New("invalid log level")
	}

	cfg.DataDir = cleanAndExpandPath(cfg.DataDir)
	if cfg.LogDir == "" {
		cfg.LogDir = cleanAndExpandPath(path.Join(cfg.DataDir, "logs"))
	}

	// Warn about missing config file only after all other configuration is
	// done.  This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		log.Errorf("%v", configFileError)
	}
	setupLogging(cfg.LogDir, cfg.LogLevel)
	return &cfg, nil
}

// createDefaultConfig copies the sample-bchd.conf content to the given destination path,
// and populates it with some randomly generated RPC username and password.
func createDefaultConfigFile(destinationPath string, testnet bool) error {
	// Create the destination directory if it does not exists
	err := os.MkdirAll(filepath.Dir(destinationPath), 0700)
	if err != nil {
		return err
	}

	sampleBytes, err := Asset("sample-obcrawler.conf")
	if err != nil {
		return err
	}
	src := bytes.NewReader(sampleBytes)

	dest, err := os.OpenFile(destinationPath,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dest.Close()

	// We copy every line from the sample config file to the destination,
	// only replacing the two lines for rpcuser and rpcpass
	reader := bufio.NewReader(src)
	for err != io.EOF {
		var line string
		line, err = reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}

		if strings.Contains(line, "bootstrapaddr=") {
			if _, err := dest.WriteString(""); err != nil {
				return err
			}
			if testnet {
				for _, addr := range defaultTestnetBootstrapAddrs {
					if _, err := dest.WriteString("bootstrapaddr=" + addr + "\n"); err != nil {
						return err
					}
				}
			} else {
				for _, addr := range DefaultMainnetBootstrapAddrs {
					if _, err := dest.WriteString("bootstrapaddr=" + addr + "\n"); err != nil {
						return err
					}
				}
			}
			continue
		}

		if _, err := dest.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(DefaultHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

func setupLogging(logDir, logLevel string) {
	backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
	backendStdoutFormatter := logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
	if logDir != "" {
		rotator := &lumberjack.Logger{
			Filename:   path.Join(logDir, defaultLogFilename),
			MaxSize:    10, // Megabytes
			MaxBackups: 3,
			MaxAge:     30, // Days
		}

		backendFile := logging.NewLogBackend(rotator, "", 0)
		backendFileFormatter := logging.NewBackendFormatter(backendFile, fileLogFormat)
		logging.SetBackend(backendStdoutFormatter, backendFileFormatter)
	} else {
		logging.SetBackend(backendStdoutFormatter)
	}
	logging.SetLevel(LogLevelMap[strings.ToLower(logLevel)], "")
}
