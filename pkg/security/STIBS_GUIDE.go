package security

/*
# STIBS - Security Threats In Backend

## What are STIBs?

STIBs (Security Threats In Backend) are subtle security vulnerabilities or backdoors
that might exist in a codebase. These can include:

1. **Hardcoded Credentials** - Credentials, API keys, or secrets embedded directly in code
2. **Weak Cryptographic Implementations** - Using weak algorithms, predictable random numbers, etc.
3. **Authentication Bypasses** - Code that allows skipping authentication under certain conditions
4. **Insecure Default Configurations** - Configurations that are insecure by default
5. **Debug Information Leaks** - Debug code that might leak sensitive information

## Common STIB Patterns

### Hardcoded Credentials
```go
const secretKey = "my-secret-key"  // Bad practice
```

### Weak Cryptographic Implementations
```go
// Using math/rand instead of crypto/rand for security purposes
rand.Seed(time.Now().UnixNano())
token := rand.Int()
```

### Authentication Bypasses
```go
// Skipping authentication in development mode
if env == "development" {
    return next.Handle(ctx)
}
```

### Insecure Default Configurations
```go
// Allow all origins by default
config := Config{
    AllowedOrigins: []string{"*"},
}
```

### Debug Information Leaks
```go
if debug {
    log.Printf("User credentials: %s:%s", username, password)
}
```

## How to Use the STIB Scanner

The STIB scanner is a tool to detect potential security issues in your codebase.

```go
// Create a new scanner
scanner := security.NewStibScanner("/path/to/codebase")

// Scan for security issues
err := scanner.Scan()
if err != nil {
    log.Fatalf("Error scanning for security issues: %v", err)
}

// Get results
results := scanner.GetResults()

// Print results
scanner.PrintResults()
```

You can also use the command-line tool:

```bash
go run scripts/tools/scan_stibs.go --path=/path/to/codebase --output=results.md --verbose
```

## Best Practices to Avoid STIBs

1. **Never hardcode credentials** - Use environment variables, secret managers, or configuration files
2. **Use secure cryptographic practices** - Use well-vetted libraries and algorithms
3. **Implement proper authentication and authorization** - Don't use bypass mechanisms
4. **Configure secure defaults** - Make security the default, not an option
5. **Be careful with debug information** - Don't log sensitive data, even in debug mode
6. **Implement proper input validation** - Validate all inputs to prevent injection attacks
7. **Use secure communication** - Use HTTPS and secure websockets
8. **Implement proper error handling** - Don't leak sensitive information in error messages

## Regular Security Audits

It's recommended to regularly scan your codebase for STIBs and other security issues.
Consider integrating the STIB scanner into your CI/CD pipeline to catch security issues early.

*/

// This file contains documentation only