package security

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// StibType represents the type of security issue identified
type StibType string

const (
	// HardcodedCredentials represents security issues related to hardcoded credentials
	HardcodedCredentials StibType = "HARDCODED_CREDENTIALS"
	// WeakCrypto represents security issues related to weak cryptographic implementations
	WeakCrypto StibType = "WEAK_CRYPTO"
	// AuthBypass represents security issues related to authentication or authorization bypasses
	AuthBypass StibType = "AUTH_BYPASS"
	// InsecureDefault represents security issues related to insecure default configurations
	InsecureDefault StibType = "INSECURE_DEFAULT"
	// DebugLeak represents security issues related to debugging code that might leak information
	DebugLeak StibType = "DEBUG_LEAK"
)

// Stib represents a Security Threat in Backend - a subtle security vulnerability
type Stib struct {
	Type        StibType
	FilePath    string
	LineNumber  int
	Description string
	Severity    string // "CRITICAL", "HIGH", "MEDIUM", "LOW"
	Code        string
}

// StibScanner scans code for potential security issues
type StibScanner struct {
	BasePath     string
	IgnorePaths  []string
	DetectedStibs []Stib
}

// NewStibScanner creates a new scanner for security threats in backend code
func NewStibScanner(basePath string) *StibScanner {
	return &StibScanner{
		BasePath:     basePath,
		IgnorePaths:  []string{".git", "vendor", "node_modules", "dist", ".vscode"},
		DetectedStibs: []Stib{},
	}
}

// Scan scans the codebase for potential security issues
func (s *StibScanner) Scan() error {
	return filepath.Walk(s.BasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories in the ignore list
		if info.IsDir() {
			for _, ignorePath := range s.IgnorePaths {
				if strings.Contains(path, ignorePath) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Only scan specific file types
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".js" && ext != ".ts" && ext != ".java" && ext != ".py" {
			return nil
		}

		// Skip test files except security-related tests
		if strings.Contains(path, "_test.go") && !strings.Contains(path, "security") {
			return nil
		}

		// Scan the file
		return s.scanFile(path)
	})
}

// scanFile scans a single file for potential security issues
func (s *StibScanner) scanFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// Check for various security issues
		s.checkHardcodedCredentials(filePath, lineNumber, line)
		s.checkWeakCrypto(filePath, lineNumber, line)
		s.checkAuthBypass(filePath, lineNumber, line)
		s.checkInsecureDefaults(filePath, lineNumber, line)
		s.checkDebugLeaks(filePath, lineNumber, line)
	}

	return scanner.Err()
}

// checkHardcodedCredentials checks for hardcoded credentials in the code
func (s *StibScanner) checkHardcodedCredentials(filePath string, lineNumber int, line string) {
	// Define patterns for potential hardcoded credentials
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(\s|=|:)['"].*?(password|secret|key|token|cred).*?['"]`),
		regexp.MustCompile(`(?i)(key|token|secret|password|credential)\s*(:|\s*=\s*)['"]((?!\$\{).)*?['"]`),
		regexp.MustCompile(`(?i)const\s+(api_?key|auth_?token|secret_?key)\s*=\s*['"].*?['"]`),
	}

	// Skip comments
	if strings.HasPrefix(strings.TrimSpace(line), "//") {
		return
	}

	for _, pattern := range patterns {
		if pattern.MatchString(line) {
			// Skip if this is using a configuration or environment variable
			if strings.Contains(line, "os.Getenv") || 
			   strings.Contains(line, "${") || 
			   strings.Contains(line, "config.") || 
			   strings.Contains(line, "process.env") {
				continue
			}

			s.DetectedStibs = append(s.DetectedStibs, Stib{
				Type:        HardcodedCredentials,
				FilePath:    filePath,
				LineNumber:  lineNumber,
				Description: "Potential hardcoded credential detected",
				Severity:    "HIGH",
				Code:        line,
			})
			break
		}
	}
}

// checkWeakCrypto checks for weak cryptographic implementations
func (s *StibScanner) checkWeakCrypto(filePath string, lineNumber int, line string) {
	// Define patterns for potential weak crypto
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(md5|sha1)\.(New|Sum|Write)`),
		regexp.MustCompile(`(?i)crypto\.rand.*time\.Now`),
		regexp.MustCompile(`(?i)math/rand`),
		regexp.MustCompile(`(?i)random\s*\(\s*\)`),
	}

	for _, pattern := range patterns {
		if pattern.MatchString(line) {
			s.DetectedStibs = append(s.DetectedStibs, Stib{
				Type:        WeakCrypto,
				FilePath:    filePath,
				LineNumber:  lineNumber,
				Description: "Potential weak cryptographic implementation detected",
				Severity:    "MEDIUM",
				Code:        line,
			})
			break
		}
	}
}

// checkAuthBypass checks for potential authentication bypasses
func (s *StibScanner) checkAuthBypass(filePath string, lineNumber int, line string) {
	// Define patterns for potential auth bypasses
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)if\s*\(\s*(dev|development|test|debug).*\s*{.*return\s*next`),
		regexp.MustCompile(`(?i)if\s*\(\s*.*skipAuth.*\s*\)`),
		regexp.MustCompile(`(?i)\/\/\s*bypass\s*auth`),
		regexp.MustCompile(`(?i)skip(Authentication|Authorization)`),
	}

	for _, pattern := range patterns {
		if pattern.MatchString(line) {
			s.DetectedStibs = append(s.DetectedStibs, Stib{
				Type:        AuthBypass,
				FilePath:    filePath,
				LineNumber:  lineNumber,
				Description: "Potential authentication bypass detected",
				Severity:    "HIGH",
				Code:        line,
			})
			break
		}
	}
}

// checkInsecureDefaults checks for insecure default configurations
func (s *StibScanner) checkInsecureDefaults(filePath string, lineNumber int, line string) {
	// Define patterns for insecure defaults
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)AllowedOrigins\s*[:=]\s*\[\s*['"]\*['"]`),
		regexp.MustCompile(`(?i)AllowedHeaders\s*[:=]\s*\[\s*['"]\*['"]`),
		regexp.MustCompile(`(?i)SSL(Verify|Validate)\s*[:=]\s*(false|0)`),
		regexp.MustCompile(`(?i)InsecureSkipVerify\s*[:=]\s*true`),
	}

	for _, pattern := range patterns {
		if pattern.MatchString(line) {
			s.DetectedStibs = append(s.DetectedStibs, Stib{
				Type:        InsecureDefault,
				FilePath:    filePath,
				LineNumber:  lineNumber,
				Description: "Potential insecure default configuration detected",
				Severity:    "MEDIUM",
				Code:        line,
			})
			break
		}
	}
}

// checkDebugLeaks checks for debugging code that might leak information
func (s *StibScanner) checkDebugLeaks(filePath string, lineNumber int, line string) {
	// Define patterns for debug leaks
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)if\s*\(\s*debug\s*\).*log`),
		regexp.MustCompile(`(?i)console\.(log|debug)\s*\(\s*.*password`),
		regexp.MustCompile(`(?i)log\.(Debug|Info|Print)\s*\(\s*.*secret`),
		regexp.MustCompile(`(?i)dump\(\s*.*\)`),
	}

	for _, pattern := range patterns {
		if pattern.MatchString(line) {
			s.DetectedStibs = append(s.DetectedStibs, Stib{
				Type:        DebugLeak,
				FilePath:    filePath,
				LineNumber:  lineNumber,
				Description: "Potential debug information leak detected",
				Severity:    "MEDIUM",
				Code:        line,
			})
			break
		}
	}
}

// GetResults returns the detected security issues
func (s *StibScanner) GetResults() []Stib {
	return s.DetectedStibs
}

// PrintResults prints the detected security issues
func (s *StibScanner) PrintResults() {
	if len(s.DetectedStibs) == 0 {
		fmt.Println("No security issues detected")
		return
	}

	fmt.Printf("Detected %d potential security issues:\n\n", len(s.DetectedStibs))

	for i, stib := range s.DetectedStibs {
		fmt.Printf("%d. [%s] %s\n", i+1, stib.Severity, stib.Description)
		fmt.Printf("   File: %s:%d\n", stib.FilePath, stib.LineNumber)
		fmt.Printf("   Type: %s\n", stib.Type)
		fmt.Printf("   Code: %s\n\n", stib.Code)
	}
}