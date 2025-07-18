package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"github.com/WJQSERVER-STUDIO/go-utils/copyb"
	"github.com/WJQSERVER-STUDIO/httpc"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"
)

// Command 定义了 CLI 的一个子命令
type Command struct {
	Name      string
	ShortDesc string
	Run       func(cmd *Command, args []string)
	fs        *flag.FlagSet
}

var commands []*Command

// 定义一个合理的默认终端输出大小限制
const defaultTerminalMaxSize = "1MB"

func init() {
	// 在这里注册所有的命令
	httpMethods := []string{"get", "post", "put", "delete", "patch", "head"}
	for _, method := range httpMethods {
		registerCommand(&Command{
			Name:      method,
			ShortDesc: fmt.Sprintf("Sends a %s request", strings.ToUpper(method)),
			Run:       handleHTTPRequest,
		})
	}
	registerCommand(&Command{
		Name:      "download",
		ShortDesc: "Downloads a file from a URL with a progress bar",
		Run:       handleDownload,
	})
	registerCommand(&Command{
		Name:      "upload",
		ShortDesc: "Uploads a file using multipart/form-data",
		Run:       handleUpload,
	})
}

func registerCommand(cmd *Command) {
	cmd.fs = flag.NewFlagSet(cmd.Name, flag.ExitOnError)
	addCommonFlags(cmd.fs)
	switch cmd.Name {
	case "download":
		cmd.fs.String("o", "", "Output file path (required).")
	case "upload":
		cmd.fs.String("file", "", "Path to the file to upload (required).")
		cmd.fs.String("field", "file", "Name of the form field for the file.")
	}
	commands = append(commands, cmd)
}

// colorScheme 定义了终端输出的颜色
type colorScheme struct{ Reset, Red, Green, Yellow, Orange, Blue, Cyan string }

var colors colorScheme

// stringSlice 是一个自定义类型, 以便让一个 flag 可以被多次使用
type stringSlice []string

func (s *stringSlice) String() string         { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(value string) error { *s = append(*s, value); return nil }

// 全局 flag 变量
var (
	timeout        time.Duration
	maxRetries     int
	userAgent      string
	verbose        bool
	includeHeaders bool
	dnsServers     stringSlice
	basicAuthUser  string
	bearerToken    string
	proxy          string
	httpProxy      string
	socks5Proxy    string
	headers        stringSlice
	formFields     stringSlice
	jsonFields     stringSlice
	rawData        string
	method         string
	maxSizeStr     string
)

func init() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		colors = colorScheme{
			Reset: "\033[0m", Red: "\033[31m", Green: "\033[32m", Yellow: "\033[33m",
			Orange: "\033[38;5;208m", Blue: "\033[34m", Cyan: "\033[36m",
		}
	}
}

func main() {
	fs := flag.NewFlagSet("hurl", flag.ExitOnError)
	addCommonFlags(fs)
	fs.StringVar(&method, "X", "", "HTTP method to use (e.g., GET, POST).")
	fs.Parse(os.Args[1:])
	args := fs.Args()

	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		if len(args) > 1 {
			if cmd := findCommand(args[1]); cmd != nil {
				cmd.fs.Usage()
				return
			}
		}
		printMainUsage()
		return
	}

	cmd := findCommand(os.Args[1])
	if cmd != nil {
		cmd.fs.Parse(os.Args[2:])
		cmd.Run(cmd, cmd.fs.Args())
	} else {
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "%sError: URL is required for no-command mode.%s\n\n", colors.Red, colors.Reset)
			printMainUsage()
			os.Exit(1)
		}
		handleNoCommand(args)
	}
}

func findCommand(name string) *Command {
	for _, cmd := range commands {
		if cmd.Name == name {
			return cmd
		}
	}
	return nil
}

func printMainUsage() {
	fmt.Fprintf(os.Stderr, `hurl is a modern command-line HTTP client to replace curl and wget.

%sUsage:%s
  hurl <command> [arguments]
  hurl <url> [flags]

%sAvailable Commands:%s
`, colors.Yellow, colors.Reset, colors.Yellow, colors.Reset)
	maxLen := 0
	for _, cmd := range commands {
		if len(cmd.Name) > maxLen {
			maxLen = len(cmd.Name)
		}
	}
	for _, cmd := range commands {
		padding := strings.Repeat(" ", maxLen-len(cmd.Name))
		fmt.Fprintf(os.Stderr, "  %s%s%s%s   %s\n", colors.Green, cmd.Name, colors.Reset, padding, cmd.ShortDesc)
	}
	fmt.Fprintf(os.Stderr, `
%sFlags:%s
  Use "hurl <command> --help" to see flags for a specific command.

%sExamples:%s
  hurl example.org
  hurl post example.org -j name=Touka
  hurl download example.org/file.zip -o my_file.zip
`, colors.Yellow, colors.Reset, colors.Yellow, colors.Reset)
}

func addCommonFlags(fs *flag.FlagSet) {
	fs.DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout.")
	fs.IntVar(&maxRetries, "retries", 2, "Maximum number of retries for failed requests.")
	fs.StringVar(&userAgent, "user-agent", "hurl/0.1 Touka HTTP Client/v0", "Set custom User-Agent.")
	fs.BoolVar(&verbose, "v", false, "Enable verbose output to see request details.")
	fs.BoolVar(&includeHeaders, "i", false, "Include response headers in the output.")
	fs.Var(&dnsServers, "dns-server", "Custom DNS server(s) (e.g., 8.8.8.8:53).")
	fs.StringVar(&basicAuthUser, "user", "", "HTTP Basic Auth ('user:pass').")
	fs.StringVar(&bearerToken, "bearer", "", "Bearer token for authentication.")
	fs.StringVar(&proxy, "x", "", "Proxy URL (e.g., 'http://...').")
	fs.StringVar(&httpProxy, "http-proxy", "", "HTTP/HTTPS proxy URL (legacy).")
	fs.StringVar(&socks5Proxy, "socks5-proxy", "", "SOCKS5 proxy URL (legacy).")
	fs.Var(&headers, "H", "Custom header(s) (e.g., 'Key:Value').")
	fs.Var(&formFields, "f", "Add a form field (key=value).")
	fs.Var(&jsonFields, "j", "Add a JSON field (key=value).")
	fs.StringVar(&rawData, "d", "", "Set raw request body from a string.")
	// 更新了帮助信息, 解释了默认行为
	fs.StringVar(&maxSizeStr, "max-size", "", "Max response size (e.g., 10KB, 5MB). Defaults to 1MB for terminal output. Use -1 for no limit.")
}

func buildClientFromFlags() (*httpc.Client, error) {
	var opts []httpc.Option
	opts = append(opts, httpc.WithTimeout(timeout), httpc.WithUserAgent(userAgent))
	if maxRetries > 0 {
		opts = append(opts, httpc.WithRetryOptions(httpc.RetryOptions{MaxAttempts: maxRetries}))
	}
	if verbose {
		opts = append(opts, httpc.WithDumpLog())
	}
	if len(dnsServers) > 0 {
		opts = append(opts, httpc.WithDNSResolver(dnsServers, 5*time.Second))
	}
	proxyToUse := proxy
	if proxyToUse != "" {
		u, err := url.Parse(proxyToUse)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyToUse, err)
		}
		switch u.Scheme {
		case "http", "https":
			opts = append(opts, httpc.WithHTTPProxy(proxyToUse))
		case "socks5", "socks5h":
			opts = append(opts, httpc.WithSocks5Proxy(proxyToUse))
		default:
			return nil, fmt.Errorf("unsupported proxy scheme: %q", u.Scheme)
		}
	} else if httpProxy != "" {
		opts = append(opts, httpc.WithHTTPProxy(httpProxy))
	}
	if socks5Proxy != "" {
		opts = append(opts, httpc.WithSocks5Proxy(socks5Proxy))
	}
	return httpc.New(opts...), nil
}

func applyRequestFlags(rb *httpc.RequestBuilder) error {
	if basicAuthUser != "" {
		rb.SetHeader("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(basicAuthUser)))
	}
	if bearerToken != "" {
		rb.SetHeader("Authorization", "Bearer "+bearerToken)
	}
	for _, h := range headers {
		key, val, _ := strings.Cut(h, ":")
		rb.AddHeader(strings.TrimSpace(key), strings.TrimSpace(val))
	}
	if rawData != "" {
		rb.SetRawBody([]byte(rawData))
	}
	if len(jsonFields) > 0 {
		body := make(map[string]interface{})
		for _, field := range jsonFields {
			key, val, _ := strings.Cut(field, "=")
			body[key] = autotype(val)
		}
		if _, err := rb.SetJSONBody(body); err != nil {
			return fmt.Errorf("failed to set json body: %w", err)
		}
	}
	if len(formFields) > 0 {
		form := url.Values{}
		for _, field := range formFields {
			key, val, _ := strings.Cut(field, "=")
			form.Add(key, val)
		}
		rb.SetHeader("Content-Type", "application/x-www-form-urlencoded")
		rb.SetBody(strings.NewReader(form.Encode()))
	}
	return nil
}

func handleNoCommand(args []string) {
	url := args[0]
	finalMethod := "GET"
	if method != "" {
		finalMethod = strings.ToUpper(method)
	} else if rawData != "" || len(jsonFields) > 0 || len(formFields) > 0 {
		finalMethod = "POST"
	}
	client, err := buildClientFromFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError building client: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	rb := client.NewRequestBuilder(finalMethod, url)
	if err := applyRequestFlags(rb); err != nil {
		fmt.Fprintf(os.Stderr, "%sError applying request flags: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	resp, err := rb.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError executing request: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	if err := processAndPrintResponse(resp); err != nil {
		fmt.Fprintf(os.Stderr, "%sError processing response: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
}

func handleHTTPRequest(cmd *Command, args []string) {
	cmd.fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: hurl %s <url> [flags]\n\nFlags for %s:\n", cmd.Name, cmd.Name)
		cmd.fs.PrintDefaults()
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%sError: URL is required for %s command.%s\n", colors.Red, cmd.Name, colors.Reset)
		cmd.fs.Usage()
		os.Exit(1)
	}
	url := args[0]
	client, err := buildClientFromFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError building client: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	rb := client.NewRequestBuilder(strings.ToUpper(cmd.Name), url)
	if err := applyRequestFlags(rb); err != nil {
		fmt.Fprintf(os.Stderr, "%sError applying request flags: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	resp, err := rb.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError executing request: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	if err := processAndPrintResponse(resp); err != nil {
		fmt.Fprintf(os.Stderr, "%sError processing response: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
}

func handleDownload(cmd *Command, args []string) {
	cmd.fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: hurl download <url> -o <output_file> [flags]\n\nFlags for download:")
		cmd.fs.PrintDefaults()
	}
	outputPath := cmd.fs.Lookup("o").Value.String()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%sError: URL is required.%s\n", colors.Red, colors.Reset)
		cmd.fs.Usage()
		os.Exit(1)
	}
	if outputPath == "" {
		fmt.Fprintf(os.Stderr, "%sError: -o flag for output file is required.%s\n", colors.Red, colors.Reset)
		cmd.fs.Usage()
		os.Exit(1)
	}
	url := args[0]
	client, err := buildClientFromFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError building client: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	rb := client.GET(url)
	applyRequestFlags(rb)
	resp, err := rb.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError starting download: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "%sError: download failed with status %s%s\n", colors.Red, resp.Status, colors.Reset)
		os.Exit(1)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError creating output file: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	defer file.Close()
	total, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	bar := progressbar.NewOptions64(total, progressbar.OptionSetDescription("Downloading"), progressbar.OptionSetWriter(os.Stderr), progressbar.OptionShowBytes(true), progressbar.OptionThrottle(65*time.Millisecond), progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stderr, "\n") }))
	maxBytes, err := parseSize(maxSizeStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError parsing max-size: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	reader := NewMaxBytesReader(resp.Body, maxBytes)
	_, err = copyb.Copy(io.MultiWriter(file, bar), reader)
	if err != nil {
		if err == ErrBodyTooLarge {
			fmt.Fprintf(os.Stderr, "\n%sError: %v (limit: %s)%s\n", colors.Red, err, maxSizeStr, colors.Reset)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "%sError writing to output file: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	fmt.Printf("%sDownloaded successfully to %s%s\n", colors.Green, outputPath, colors.Reset)
}

func handleUpload(cmd *Command, args []string) {
	cmd.fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: hurl upload <url> --file <path> [flags]\n\nFlags for upload:")
		cmd.fs.PrintDefaults()
	}
	filePath := cmd.fs.Lookup("file").Value.String()
	fieldName := cmd.fs.Lookup("field").Value.String()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%sError: URL is required.%s\n", colors.Red, colors.Reset)
		cmd.fs.Usage()
		os.Exit(1)
	}
	if filePath == "" {
		fmt.Fprintf(os.Stderr, "%sError: --file flag is required.%s\n", colors.Red, colors.Reset)
		cmd.fs.Usage()
		os.Exit(1)
	}
	url := args[0]
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError opening file: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	defer file.Close()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(fieldName, filepath.Base(filePath))
	copyb.Copy(part, file)
	for _, f := range formFields {
		key, val, _ := strings.Cut(f, "=")
		writer.WriteField(key, val)
	}
	writer.Close()
	client, err := buildClientFromFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError building client: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	rb := client.POST(url).SetBody(body)
	rb.SetHeader("Content-Type", writer.FormDataContentType())
	applyRequestFlags(rb)
	resp, err := rb.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError executing request: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
	if err := processAndPrintResponse(resp); err != nil {
		fmt.Fprintf(os.Stderr, "%sError processing response: %v%s\n", colors.Red, err, colors.Reset)
		os.Exit(1)
	}
}

func processAndPrintResponse(resp *http.Response) error {
	// --- 智能大小限制逻辑 ---
	effectiveMaxSizeStr := maxSizeStr
	isDefaultLimit := false
	// 如果用户没有设置 --max-size 并且输出是终端
	if maxSizeStr == "" && isatty.IsTerminal(os.Stdout.Fd()) {
		effectiveMaxSizeStr = defaultTerminalMaxSize
		isDefaultLimit = true
	}

	maxBytes, err := parseSize(effectiveMaxSizeStr)
	if err != nil {
		return err
	}

	limitedBody := NewMaxBytesReader(resp.Body, maxBytes)
	defer limitedBody.Close()

	if includeHeaders || resp.Request.Method == http.MethodHead {
		var statusColor string
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			statusColor = colors.Green
		case resp.StatusCode >= 300 && resp.StatusCode < 400:
			statusColor = colors.Yellow
		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			statusColor = colors.Orange
		default:
			statusColor = colors.Red
		}
		fmt.Printf("%s%s%s %s%s%s\n", colors.Cyan, resp.Proto, colors.Reset, statusColor, resp.Status, colors.Reset)
		for key, values := range resp.Header {
			fmt.Printf("%s%s%s: %s\n", colors.Blue, key, colors.Reset, strings.Join(values, ", "))
		}
		fmt.Println()
	}
	if resp.Request.Method == http.MethodHead {
		return nil
	}

	body, err := copyb.ReadAll(limitedBody)
	if err != nil {
		if err == ErrBodyTooLarge {
			if isDefaultLimit {
				return fmt.Errorf("%w (default terminal limit: %s). Use --max-size to override.", err, defaultTerminalMaxSize)
			}
			return fmt.Errorf("%w (limit: %s)", err, maxSizeStr)
		}
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		var v any
		if err := json.Unmarshal(body, &v); err == nil {
			prettyJSON, err := json.Marshal(v, jsontext.Multiline(true), jsontext.WithIndent("  "))
			if err == nil {
				fmt.Println(string(prettyJSON))
				return nil
			}
		}
	}
	fmt.Println(string(body))
	return nil
}

func autotype(val string) interface{} {
	if i, err := strconv.ParseInt(val, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(val); err == nil {
		return b
	}
	return val
}

func parseSize(sizeStr string) (int64, error) {
	if sizeStr == "-1" || sizeStr == "" {
		return -1, nil
	}
	re := regexp.MustCompile(`(?i)^(\d+)\s*(k|m|g|t)?b?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(sizeStr))
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}
	val, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}
	unit := strings.ToLower(matches[2])
	switch unit {
	case "k":
		val *= 1 << 10
	case "m":
		val *= 1 << 20
	case "g":
		val *= 1 << 30
	case "t":
		val *= 1 << 40
	}
	return val, nil
}
