package internal

import (
	"errors"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zhangzqs/curl-go/internal/version"
)

type Flags struct {
	// Verbose mode
	Verbose bool
	// OutputRequestBodyOnVerbose 用于调试时，输出请求body
	OutputRequestBodyOnVerbose bool
	// OutputResponseBodyOnVerbose 用于调试时，输出响应body
	OutputResponseBodyOnVerbose bool

	URL string

	UserAgent string

	// Request Method
	Request string

	Header []string

	FormEntry []string

	// Body
	Data string

	// output pretty response body
	Pretty bool

	// Save response body to file
	OutputFile string

	// 自动计算并添加content-md5请求头
	ContentMD5 bool

	// 是否输出版本信息
	Version bool

	// Output response headers
	DumpHeader string

	// 使用代理[protocol://]host[:port]
	Proxy string

	// --trace使用TRACE级别日志
	Trace bool

	// 关闭所有日志输出
	Silent bool

	// Trailer
	Trailer []string

	// -I Head
	Head bool

	// -m / --max-time
	MaxTime float64
	// --connect-timeout
	ConnectTimeout float64
}

func (f *Flags) validateMethodFlag() error {
	methods := []string{
		http.MethodHead,
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace,
	}
	for _, method := range methods {
		if strings.ToUpper(f.Request) == method {
			return nil
		}
	}
	return errors.New("invalid method: " + f.Request + ", valid methods: " + strings.Join(methods, ", "))
}

func (f *Flags) ValidateAndFillDefault() (err error) {
	// 默认method填充
	if f.Request == "" {
		// 如果有表单且未修改默认 method 为 POST，自动修改 method 为 POST
		switch {
		case len(f.FormEntry) > 0:
			f.Request = http.MethodPost
		case f.Data != "":
			f.Request = http.MethodPost
		case f.Head:
			f.Request = http.MethodHead
		default:
			f.Request = http.MethodGet
		}
	} else {
		// 有method就校验正确性
		if err = f.validateMethodFlag(); err != nil {
			return
		}
	}
	return nil
}

func (f *Flags) RegisterForCommand(cmd *cobra.Command) {
	cmd.Flags().SortFlags = false

	// verbose
	{
		cmd.Flags().BoolVar(&f.OutputRequestBodyOnVerbose, "verbose-req-body", false, "Output request body on verbose mode")
		cmd.Flags().BoolVar(&f.OutputResponseBodyOnVerbose, "verbose-resp-body", false, "Output response body on verbose mode")
	}

	// 这几个是标准curl的flags
	{
		cmd.Flags().StringVar(&f.URL, "url", "", "Request url")
		cmd.Flags().StringVarP(&f.UserAgent, "user-agent", "A", version.GetDefaultUserAgent(), "Set header User-Agent")
		cmd.Flags().StringVarP(&f.Request, "request", "X", "", "Request Method (GET|POST|PUT|DELETE|HEAD|OPTIONS|PATCH)")
		cmd.Flags().StringSliceVarP(&f.Header, "header", "H", []string{}, `Header (key:value), for example: "Content-Type:application/json", "Content-Type:application/xml", "Content-Type:application/octet-stream", "Content-Type:application/x-www-form-urlencoded"`)

		// request body from
		{
			cmd.Flags().StringVarP(&f.Data, "data", "d", "", "Body data, use @filename to read from file")
			cmd.Flags().StringSliceVarP(&f.FormEntry, "form", "F", []string{}, "Form data (key=value), use @filename to read from file")
			cmd.MarkFlagsMutuallyExclusive("data", "form")
		}

		cmd.Flags().StringVarP(&f.OutputFile, "output", "o", "", "Save response body to file")

		// Version
		cmd.Flags().BoolVarP(&f.Version, "version", "V", false, "Output version info")

		// DumpHeader
		cmd.Flags().StringVarP(&f.DumpHeader, "dump-header", "D", "", "Output response headers to file")

		// Proxy
		cmd.Flags().StringVarP(&f.Proxy, "proxy", "x", "", "Use proxy [protocol://]host[:port]")

		// Head
		cmd.Flags().BoolVarP(&f.Head, "head", "I", false, "Default use head request, only print response headers")

		// Logger
		{
			// Verbose
			cmd.Flags().BoolVarP(&f.Verbose, "verbose", "v", false, "Verbose mode, use debug log level, output verbose info, include request and response headers")

			// Trace
			cmd.Flags().BoolVar(&f.Trace, "trace", false, "Output all trace info, use trace log level")

			// Silent
			cmd.Flags().BoolVarP(&f.Silent, "silent", "s", false, "Silent mode, discard all log")
		}

		// Timeout 总超时时间
		cmd.Flags().Float64VarP(&f.MaxTime, "max-time", "m", 0, "<fractional seconds> Maximum time allowed for http request")
		// Connect timeout TCP连接超时时间
		cmd.Flags().Float64Var(&f.ConnectTimeout, "connect-timeout", 30.0, "<fractional seconds> Maximum time allowed for connection")
	}

	// output pretty response json body
	cmd.Flags().BoolVarP(&f.Pretty, "pretty", "p", false, "Output pretty response body, only for json response")

	// Content-MD5
	cmd.Flags().BoolVar(&f.ContentMD5, "content-md5", false, "Auto calculate request body content md5 and add Content-MD5 header or trailer(if Transfer-Encoding:chunked)")

	// http trailer
	cmd.Flags().StringSliceVar(&f.Trailer, "trailer", []string{}, "Trailer (key:value)")
}
