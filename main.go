package main

import (
	"embed"
	"os"

	"github.com/MrCodeEU/LinuxHello/internal/cli"
	"github.com/MrCodeEU/LinuxHello/internal/daemon"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "daemon":
			daemon.Run(os.Args[2:])
			return
		case "enroll":
			cli.RunEnroll(os.Args[2:])
			return
		case "test":
			cli.RunTest(os.Args[2:])
			return
		case "gui":
			// Explicit GUI subcommand, require root
			requireRoot()
		case "--help", "-h", "help":
			printHelp()
			return
		case "--version", "-v", "version":
			printVersion()
			return
		default:
			// Unknown args (e.g. Wails internal flags like binding generation)
			// pass through to runWailsApp() without root check
		}
	} else {
		// No args = launch GUI, require root
		requireRoot()
	}

	runWailsApp()
}

func requireRoot() {
	if os.Geteuid() != 0 {
		println("Error: LinuxHello GUI requires root/sudo for camera access and PAM management.")
		println("Please run: sudo linuxhello")
		os.Exit(1)
	}
}

func runWailsApp() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "LinuxHello",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 23, G: 23, B: 23, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			ProgramName: "LinuxHello",
		},
	})

	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
}

func printHelp() {
	println("LinuxHello - Linux Face Authentication System")
	println("")
	println("Usage:")
	println("  linuxhello              Run the GUI application (default)")
	println("  linuxhello gui          Run the GUI application (explicit)")
	println("  linuxhello daemon       Run as background daemon")
	println("  linuxhello enroll       Enroll a user's face")
	println("  linuxhello test         Test face authentication")
	println("  linuxhello --help       Show this help message")
	println("  linuxhello --version    Show version information")
	println("")
	println("Subcommand Options:")
	println("")
	println("  daemon:")
	println("    -config <path>        Path to configuration file")
	println("    -verbose              Enable verbose logging")
	println("")
	println("  enroll:")
	println("    -user <username>      Username to enroll (required)")
	println("    -samples <n>          Number of samples to capture (default: 5)")
	println("    -config <path>        Path to configuration file")
	println("    -list                 List enrolled users")
	println("    -delete <username>    Delete user enrollment")
	println("")
	println("  test:")
	println("    -user <username>      Specific user to authenticate (optional)")
	println("    -config <path>        Path to configuration file")
	println("    -continuous           Continuous authentication mode")
	println("")
	println("Examples:")
	println("  sudo linuxhello                        # Run GUI")
	println("  sudo linuxhello daemon                 # Run as daemon")
	println("  sudo linuxhello enroll -user john      # Enroll user john")
	println("  sudo linuxhello test                   # Test authentication")
}

func printVersion() {
	println("LinuxHello - Linux Face Authentication System")
	println("========================================")
	println("Version: 1.3.4")
	println("License: MIT")
	println("")
	println("Features:")
	println("  - SCRFD face detection")
	println("  - ArcFace recognition")
	println("  - Depth-based liveness detection")
	println("  - Challenge-response authentication")
	println("  - PAM integration")
	println("")
	println("Hardware Support:")
	println("  - V4L2 cameras (RGB)")
	println("  - IR cameras")
	println("  - Intel RealSense depth cameras")
}
