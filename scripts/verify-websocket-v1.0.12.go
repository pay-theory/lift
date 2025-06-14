package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("=== Lift v1.0.12 WebSocket Verification Script ===")

	// Check Go version
	fmt.Println("1. Checking Go version...")
	goVersion, err := exec.Command("go", "version").Output()
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ✅ %s", goVersion)
	}

	// Check if Lift is installed
	fmt.Println("\n2. Checking Lift installation...")
	modList, err := exec.Command("go", "list", "-m", "github.com/pay-theory/lift").Output()
	if err != nil {
		fmt.Printf("   ❌ Lift not found in go.mod\n")
		fmt.Println("   Run: go get github.com/pay-theory/lift@v1.0.12")
	} else {
		version := strings.TrimSpace(string(modList))
		if strings.Contains(version, "v1.0.12") {
			fmt.Printf("   ✅ %s\n", version)
		} else {
			fmt.Printf("   ⚠️  Wrong version: %s\n", version)
			fmt.Println("   Run: go get github.com/pay-theory/lift@v1.0.12")
		}
	}

	// Create a test file
	fmt.Println("\n3. Testing WebSocket API...")
	testCode := `package main

import (
	"fmt"
	"github.com/pay-theory/lift/pkg/lift"
)

func main() {
	// Test WithWebSocketSupport
	app := lift.New(lift.WithWebSocketSupport())
	fmt.Println("✅ WithWebSocketSupport works!")
	
	// Test WebSocket method
	app.WebSocket("$connect", func(ctx *lift.Context) error {
		return nil
	})
	fmt.Println("✅ app.WebSocket() works!")
	
	// Test WebSocketHandler
	handler := app.WebSocketHandler()
	if handler != nil {
		fmt.Println("✅ app.WebSocketHandler() works!")
	}
}
`

	// Write test file
	err = os.WriteFile("test_websocket.go", []byte(testCode), 0644)
	if err != nil {
		fmt.Printf("   ❌ Could not create test file: %v\n", err)
		return
	}
	defer os.Remove("test_websocket.go")

	// Try to build it
	fmt.Println("   Building test file...")
	buildCmd := exec.Command("go", "build", "-o", "test_websocket", "test_websocket.go")
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("   ❌ Build failed:\n%s\n", buildOutput)
		fmt.Println("\n   Troubleshooting steps:")
		fmt.Println("   1. Run: go clean -modcache")
		fmt.Println("   2. Run: go get github.com/pay-theory/lift@v1.0.12")
		fmt.Println("   3. Run: go mod tidy")
	} else {
		fmt.Println("   ✅ Build successful!")
		defer os.Remove("test_websocket")

		// Run the test
		fmt.Println("   Running test...")
		runCmd := exec.Command("./test_websocket")
		runOutput, err := runCmd.Output()
		if err != nil {
			fmt.Printf("   ❌ Run failed: %v\n", err)
		} else {
			fmt.Printf("%s", runOutput)
		}
	}

	// Check for required dependencies
	fmt.Println("\n4. Checking required dependencies...")
	deps := []string{
		"github.com/aws/aws-lambda-go",
		"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi",
	}

	for _, dep := range deps {
		modCheck, err := exec.Command("go", "list", "-m", dep).Output()
		if err != nil {
			fmt.Printf("   ❌ Missing: %s\n", dep)
			fmt.Printf("      Run: go get %s\n", dep)
		} else {
			fmt.Printf("   ✅ %s", modCheck)
		}
	}

	fmt.Println("\n=== Summary ===")
	fmt.Println("If all checks passed, WebSocket support is properly installed.")
	fmt.Println("If any checks failed, follow the troubleshooting steps above.")
}
