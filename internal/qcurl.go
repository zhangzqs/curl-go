package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-xmlfmt/xmlfmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tidwall/pretty"
	"github.com/zhangzqs/curl-go/internal/version"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
)

var curlFlag = &Flags{}

func init() {
	curlFlag.RegisterForCommand(cmd)
}

func fillBody(req *http.Request) error {
	isChunked := req.Header.Get("Transfer-Encoding") == "chunked"

	// set body
	if strings.HasPrefix(curlFlag.Data, "@") {
		// file data
		filename := curlFlag.Data[1:]
		isStdin := filename == "-"
		log.Trace("set body content from file: " + filename)
		if isStdin {
			req.Body = os.Stdin
		} else {
			f, err := os.Open(filename)
			if err != nil {
				return err
			}
			req.Body = f
			if isChunked {
				req.ContentLength = -1
			} else {
				stat, err := f.Stat()
				if err != nil {
					return err
				}
				req.ContentLength = stat.Size()
			}

			if curlFlag.ContentMD5 {
				f, err := os.Open(filename)
				if err != nil {
					return err
				}
				md5, err := GetBase64MD5FromReader(f)
				if err != nil {
					return err
				}
				f.Close()
				if isChunked {
					if req.Trailer == nil {
						req.Trailer = make(http.Header)
					}
					// chunked 传输在 trailer 添加 content-md5
					req.Trailer.Set("Content-MD5", md5)
					log.Trace("add trailer: Content-MD5: " + md5)
				} else {
					// 非 chunked 传输在 header 添加 content-md5
					req.Header.Set("Content-MD5", md5)
					log.Trace("add header: Content-MD5: " + md5)
				}
			}
		}
	} else if curlFlag.Data != "" {
		// raw data
		log.Trace("set body content: " + curlFlag.Data)
		req.Body = io.NopCloser(strings.NewReader(curlFlag.Data))
		req.ContentLength = int64(len(curlFlag.Data))
		if curlFlag.ContentMD5 {
			md5 := GetBase64MD5FromStr(curlFlag.Data)
			if md5 == "" {
				return fmt.Errorf("getBase64MD5FromStr error")
			}
			if isChunked {
				if req.Trailer == nil {
					req.Trailer = make(http.Header)
				}
				req.Trailer.Set("Content-MD5", md5)
				log.Trace("add trailer: Content-MD5: " + md5)
			} else {
				req.Header.Set("Content-MD5", md5)
				log.Trace("add header: Content-MD5: " + md5)
			}
		}
	} else if len(curlFlag.FormEntry) != 0 {
		// form data
		BuildFormData(req, curlFlag.FormEntry)
	}
	return nil
}

func buildUnsignedRequest(urlStr string) (*http.Request, error) {
	urlStr = strings.TrimSpace(urlStr)
	// 填充默认协议 http
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}
	_, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %s (%s)", urlStr, err.Error())
	}
	req, err := http.NewRequest(curlFlag.Request, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid request: %s", err.Error())
	}

	// set header
	if curlFlag.UserAgent != "" {
		req.Header.Set("User-Agent", curlFlag.UserAgent)
		log.Trace("add header: User-Agent: " + curlFlag.UserAgent)
	}

	// 填充content-type
	if curlFlag.Data != "" {
		// 如果-d参数不为空，自动填充content-type
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		log.Trace("add header: Content-Type: application/x-www-form-urlencoded")
	}

	// 解析并添加body
	if err := fillBody(req); err != nil {
		return nil, err
	}

	// 最终用户显式输入的header优先级最高，可覆盖先前默认逻辑填充的header
	if len(curlFlag.Header) > 0 {
		// extra headers
		for _, h := range curlFlag.Header {
			idx := strings.Index(h, ":")
			if idx == -1 {
				return nil, fmt.Errorf("invalid header: %s", h)
			} else {
				k := strings.TrimSpace(h[:idx])
				v := strings.TrimSpace(h[idx+1:])
				req.Header.Set(k, v)
				log.Tracef("add header: %s: %s", k, v)
			}
		}
		// set host header
		if host := req.Header.Get("Host"); host != "" {
			req.Host = host
			log.Trace("set header Host:" + host)
		}
		// set content-length
		if contentLength := req.Header.Get("Content-Length"); contentLength != "" {
			// 转换为int64
			l, err := strconv.ParseInt(contentLength, 10, 64)
			if err != nil {
				err = fmt.Errorf("parse content-length header value error: %s", contentLength)
				log.Error(err)
				return nil, err
			}
			req.ContentLength = l
			log.Tracef("set content-length: %d", l)
		}
	}

	// Add trailer
	if len(curlFlag.Trailer) != 0 {
		if req.Trailer == nil {
			req.Trailer = make(http.Header)
		}
		for _, t := range curlFlag.Trailer {
			if idx := strings.Index(t, ":"); idx == -1 {
				return nil, fmt.Errorf("invalid trailer: %s", t)
			} else {
				k := strings.TrimSpace(t[:idx])
				v := strings.TrimSpace(t[idx+1:])
				req.Trailer.Set(k, v)
				log.Tracef("add trailer: %s: %s", k, v)
			}
		}
	}
	return req, nil
}

func outputResponse(resp *http.Response) error {
	// output header
	if curlFlag.DumpHeader != "" || curlFlag.Head {
		bs, err := httputil.DumpResponse(resp, false)
		if err != nil {
			log.Error("httputil.DumpResponse error: ", err)
			return err
		}

		if curlFlag.DumpHeader != "" {
			// 有指定输出文件，输出到文件
			if err := os.WriteFile(curlFlag.DumpHeader, bs, 0644); err != nil {
				log.Error("write dump header file error: ", err)
				return err
			}
		}
		if curlFlag.Head {
			// Head请求输出到控制台
			fmt.Print(string(bs))
		}
	}

	if !curlFlag.Head && log.GetLevel() >= log.DebugLevel {
		logPrefix := "print response without body: "
		if curlFlag.OutputResponseBodyOnVerbose {
			logPrefix = "print response with body: "
		}
		bs, err := httputil.DumpResponse(resp, curlFlag.OutputResponseBodyOnVerbose)
		if err != nil {
			log.Error("httputil.DumpResponse error: ", err)
			return err
		}
		log.Infoln(logPrefix + "\n" + string(bs))
	}

	// don't output body if content-length == 0
	if resp.ContentLength == 0 {
		return nil
	}

	// output body
	outputRaw := func(r io.Reader) error {
		if curlFlag.OutputFile == "" {
			if _, err := io.Copy(os.Stdout, r); err != nil {
				return err
			}
			return nil
		}

		// Save file is not empty, save response body to file
		if f, err := os.Create(curlFlag.OutputFile); err != nil {
			return err
		} else {
			if _, err := io.Copy(f, r); err != nil {
				return err
			} else {
				log.Println("save file success: " + curlFlag.OutputFile)
			}
			return nil
		}
	}

	// Output response body directly
	if !curlFlag.Pretty {
		return outputRaw(resp.Body)
	}

	outputPrettyXML := func(bs []byte) (err error) {
		if len(bs) == 0 {
			return
		}
		var file *os.File
		if curlFlag.OutputFile != "" {
			file, err = os.Create(curlFlag.OutputFile)
			if err != nil {
				return err
			}
		} else {
			file = os.Stdout
		}
		_, err = file.WriteString(xmlfmt.FormatXML(string(bs), "", "    ", true))
		if err != nil {
			return err
		}
		return
	}

	outputPrettyJson := func(bs []byte) (err error) {
		if len(bs) == 0 {
			return
		}
		var file *os.File
		if curlFlag.OutputFile != "" {
			file, err = os.Create(curlFlag.OutputFile)
			if err != nil {
				return err
			}
		} else {
			file = os.Stdout
		}

		stat, err := file.Stat()
		if err != nil {
			return err
		}

		var prettyJson []byte
		if stat.Mode()&os.ModeCharDevice != 0 {
			// 终端
			prettyJson = pretty.Color(bs, pretty.TerminalStyle)
		} else {
			// 文件
			prettyJson = pretty.Pretty(bs)
		}

		_, err = file.Write(prettyJson)
		if err != nil {
			return err
		}
		return
	}

	// Output response body with pretty format
	contentType := resp.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentType, "application/json"):
		var body any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return err
		}
		if bs, err := json.MarshalIndent(body, "", "   "); err != nil {
			return err
		} else {
			return outputPrettyJson(bs)
		}
	case strings.HasPrefix(contentType, "application/bson"):
		// 读取bson转化为json并格式化输出
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		decoder, err := bson.NewDecoder(bsonrw.NewBSONDocumentReader(bs))
		if err != nil {
			return err
		}
		var body bson.M
		if err := decoder.Decode(&body); err != nil {
			return err
		}
		if bs, err := json.MarshalIndent(body, "", "   "); err != nil {
			return err
		} else {
			return outputPrettyJson(bs)
		}
	case strings.HasPrefix(contentType, "application/xml"):
		// 读取xml并格式化输出
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return outputPrettyXML(bs)
	default:
		log.Warnf("pretty not support %s", contentType)
		return outputRaw(resp.Body)
	}
}

func outputRequest(req *http.Request) error {
	// Print request message
	if log.GetLevel() < log.DebugLevel {
		return nil
	}
	bs, err := httputil.DumpRequestOut(req, false)
	if err != nil {
		log.Errorf("dump request error: %v", err)
		return err
	}

	withBody := req.Body != nil && curlFlag.OutputRequestBodyOnVerbose

	logPrefix := "print request without body: "
	if withBody {
		logPrefix = "print request with body: "
	}

	log.Infoln(logPrefix + "\n" + string(bs))
	if withBody {
		srcReqBody := req.Body
		pipeReader, pipeWriter := io.Pipe()
		req.Body = pipeReader
		copyReader := io.TeeReader(srcReqBody, pipeWriter)
		go func() {
			var err error
			defer srcReqBody.Close()
			defer pipeWriter.CloseWithError(err)
			_, err = io.Copy(os.Stderr, copyReader)
			if err != nil {
				log.Error("copy body to stderr err: ", err)
				return
			}
		}()
	}
	return err
}

var cmd = &cobra.Command{
	Use:          "curl-go <url>",
	Short:        "curl-go is a tool to send raw http request",
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if err = curlFlag.ValidateAndFillDefault(); err != nil {
			_ = cmd.Usage()
		}
		return
	},

	RunE: func(cmd *cobra.Command, args []string) error {
		if curlFlag.Version {
			version.PrintVersionInfo()
			return nil
		}

		switch {
		case curlFlag.Silent:
			log.SetOutput(io.Discard)
		case curlFlag.Trace:
			log.SetLevel(log.TraceLevel)
		case curlFlag.Verbose:
			log.SetLevel(log.DebugLevel)
		default:
			log.SetLevel(log.InfoLevel)
		}

		// set url
		var urlStr string
		switch {
		case len(args) == 1:
			urlStr = args[0]
		case len(args) > 1:
			return errors.New("too many arguments")
		case curlFlag.URL != "":
			urlStr = curlFlag.URL
		default:
			return errors.New("url argument is required")
		}

		// build request
		log.Trace("build request: " + urlStr)
		req, err := buildUnsignedRequest(urlStr)
		if err != nil {
			return err
		}

		// send request and receive response
		c := http.Client{
			Transport: &http.Transport{
				Proxy: func(req *http.Request) (*url.URL, error) {
					if curlFlag.Proxy != "" {
						return url.Parse(curlFlag.Proxy)
					}
					return http.ProxyFromEnvironment(req)
				},
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					dialer := &net.Dialer{
						Timeout:   time.Duration(curlFlag.ConnectTimeout * float64(time.Second)),
						KeepAlive: 30 * time.Second,
						Resolver:  net.DefaultResolver,
					}
					return dialer.DialContext(ctx, network, addr)
				},
			},
			Timeout: time.Duration(curlFlag.MaxTime * float64(time.Second)),
		}

		// output request
		if err := outputRequest(req); err != nil {
			return err
		}

		if curlFlag.Trace {
			req = req.WithContext(httptrace.WithClientTrace(req.Context(), BuildClientTrace()))
		}

		resp, err := c.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// output response
		if err := outputResponse(resp); err != nil {
			return err
		}

		if resp.StatusCode >= 400 {
			fmt.Println()
			err = fmt.Errorf("request failed with response status code: %d", resp.StatusCode)
			log.Error(err)
			return err
		}

		return nil
	},
}

func Execute() error {
	return cmd.Execute()
}
