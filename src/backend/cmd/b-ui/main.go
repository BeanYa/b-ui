package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/BeanYa/b-ui/src/backend/internal/app"
	cmd "github.com/BeanYa/b-ui/src/backend/internal/cli"
)

func runApp() {
	app := app.NewApp()

	err := app.Init()
	if err != nil {
		log.Fatal(err)
	}

	err = app.Start()
	if err != nil {
		log.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	// Trap shutdown signals
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)
	for {
		sig := <-sigCh

		switch sig {
		case syscall.SIGHUP:
			app.RestartApp()
		default:
			app.Stop()
			return
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		runApp()
		return
	} else if os.Args[1] == "run" || isRunFlag(os.Args[1]) {
		parseRunArgs()
		runApp()
		return
	} else {
		cmd.ParseCmd()
	}
}

func isRunFlag(arg string) bool {
	return arg == "-default-admin-username" ||
		arg == "--default-admin-username" ||
		arg == "-default-admin-password" ||
		arg == "--default-admin-password"
}

func parseRunArgs() {
	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	var defaultAdminUsername string
	var defaultAdminPassword string
	runCmd.StringVar(&defaultAdminUsername, "default-admin-username", "", "set first admin username at startup")
	runCmd.StringVar(&defaultAdminPassword, "default-admin-password", "", "set first admin password at startup")

	args := os.Args[1:]
	if args[0] == "run" {
		args = args[1:]
	}
	if err := runCmd.Parse(args); err != nil {
		log.Fatal(err)
	}
	if defaultAdminUsername != "" {
		_ = os.Setenv("BUI_DEFAULT_ADMIN_USERNAME", defaultAdminUsername)
	}
	if defaultAdminPassword != "" {
		_ = os.Setenv("BUI_DEFAULT_ADMIN_PASSWORD", defaultAdminPassword)
	}
}
