# IPv4 Subnet Calculator

A simple, lightweight web-based IPv4 subnet calculator built with Go. This application provides an easy-to-use interface for calculating subnet information including network addresses, broadcast addresses, host ranges, and the number of usable hosts.

## Features

### Core Functionality
- **Network Address Calculation**: Determines the first IP address in a subnet
- **Broadcast Address Calculation**: Determines the last IP address in a subnet  
- **Host Range Calculation**: Provides minimum and maximum host addresses
- **Usable Host Count**: Calculates the total number of usable IP addresses in the subnet
- **Flexible Input**: Supports both CIDR notation (/24) and dotted decimal notation (255.255.255.0) for subnet masks

### Technical Features
- Built with Go's standard library (no external dependencies)
- Clean separation of HTML templates and Go logic
- Environment variable configuration for port selection
- Comprehensive error handling and input validation
- Responsive web interface with modern styling

## Installation

### Prerequisites
- Go 1.19 or later
- Web browser

### Quick Start

1. **Clone or download the application files**:
   ```bash
   git clone git@github.com:jurikolo/go-ip-subnet-calculator.git
   ```

2. **Run the application**:
   ```bash
   go run main.go
   ```

3. **Access the application**:
   Open your web browser and navigate to `http://localhost:8080`

## Configuration

### Port Configuration
The application uses the `GO_SUBNET_CALCULATOR_PORT` environment variable to determine which port to run on. If not set, it defaults to port 8080.

**Examples:**
```bash
# Run on default port 8080
go run main.go

# Run on custom port 3000
GO_SUBNET_CALCULATOR_PORT=3000 go run main.go

# Run on port 80 (requires admin privileges on most systems)
sudo GO_SUBNET_CALCULATOR_PORT=80 go run main.go
```

## Usage

### Basic Usage

1. **Enter IP Address**: Input any valid IPv4 address (e.g., `192.168.1.100`)
2. **Enter Subnet Mask**: Use either format:
   - CIDR notation: `/24`, `/16`, `/30`, etc.
   - Dotted decimal: `255.255.255.0`, `255.255.0.0`, etc.
3. **Click Calculate**: View the comprehensive subnet information

### Input Examples

| IP Address | Subnet Mask | Description |
|------------|-------------|-------------|
| `192.168.1.100` | `/24` | Standard home/office network |
| `10.0.5.20` | `255.255.0.0` | Class B private network |
| `172.16.1.50` | `/30` | Point-to-point connection (2 hosts) |
| `203.0.113.10` | `/32` | Single host |
| `198.51.100.5` | `/31` | Point-to-point link (no host IPs) |

### Sample Output

For IP `192.168.1.100` with subnet mask `/24`:

```
Network Address:         192.168.1.0
Broadcast Address:       192.168.1.255
Min Host Address:        192.168.1.1
Max Host Address:        192.168.1.254
Number of Usable Hosts:  254
```

## Development

### Project Structure
```
subnet-calculator/
├── main.go           # Main application logic
├── index.html        # HTML template
├── main_test.go      # Unit tests
└── README.md         # This file
```

### Running Tests
```bash
# Run all tests
go test

# Run tests with verbose output
go test -v

# Run tests with coverage
go test -cover

# Run benchmarks
go test -bench=.

# Run specific test
go test -run TestCalculateSubnet
```

### Code Coverage
```bash
# Generate coverage report
go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Building for Production
```bash
# Build for current platform
go build -o subnet-calculator

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o subnet-calculator-linux

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o subnet-calculator.exe

# Build for macOS
GOOS=darwin GOARCH=amd64 go build -o subnet-calculator-mac
```

## Deployment

### Local Development
```bash
go run main.go
```

### Production Server
```bash
# Build the application
go build -o subnet-calculator

# Run with custom port
GO_SUBNET_CALCULATOR_PORT=8080 ./subnet-calculator
```
