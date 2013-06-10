// This is the main package for the `packer` application.
package main

import (
	"bytes"
	"fmt"
	"github.com/mitchellh/packer/packer"
	"github.com/mitchellh/packer/packer/plugin"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

func main() {
	if os.Getenv("PACKER_LOG") == "" {
		// If we don't have logging explicitly enabled, then disable it
		log.SetOutput(ioutil.Discard)
	} else {
		// Logging is enabled, make sure it goes to stderr
		log.SetOutput(os.Stderr)
	}

	// If there is no explicit number of Go threads to use, then set it
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	config, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: \n\n%s\n", err)
		os.Exit(1)
	}

	log.Printf("Packer config: %+v", config)

	defer plugin.CleanupClients()

	var cache packer.Cache
	if cacheDir := os.Getenv("PACKER_CACHE_DIR"); cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error preparing cache directory: \n\n%s\n", err)
			os.Exit(1)
		}

		log.Printf("Setting cache directory: %s", cacheDir)
		cache = &packer.FileCache{CacheDir: cacheDir}
	}

	envConfig := packer.DefaultEnvironmentConfig()
	envConfig.Cache = cache
	envConfig.Commands = config.CommandNames()
	envConfig.Components.Builder = config.LoadBuilder
	envConfig.Components.Command = config.LoadCommand
	envConfig.Components.Hook = config.LoadHook
	envConfig.Components.Provisioner = config.LoadProvisioner

	env, err := packer.NewEnvironment(envConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Packer initialization error: \n\n%s\n", err)
		os.Exit(1)
	}

	setupSignalHandlers(env)

	exitCode, err := env.Cli(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
		os.Exit(1)
	}

	plugin.CleanupClients()
	os.Exit(exitCode)
}

func loadConfig() (*config, error) {
	var config config
	if err := decodeConfig(bytes.NewBufferString(defaultConfig), &config); err != nil {
		return nil, err
	}

	mustExist := true
	configFile := os.Getenv("PACKER_CONFIG")
	if configFile == "" {
		u, err := user.Current()
		if err != nil {
			return nil, err
		}

		configFile = filepath.Join(u.HomeDir, ".packerrc")
		mustExist = false
	}

	log.Printf("Attempting to open config file: %s", configFile)
	f, err := os.Open(configFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		if mustExist {
			return nil, err
		}

		log.Println("File doesn't exist, but doesn't need to. Ignoring.")
		return &config, nil
	}
	defer f.Close()

	if err := decodeConfig(f, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
