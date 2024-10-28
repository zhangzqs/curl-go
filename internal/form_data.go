package internal

import (
	"bufio"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

type FormBodyBuilder struct {
	w *multipart.Writer
}

func (b *FormBodyBuilder) createFormPart(fieldname string, filename *string, contentType *string) (io.Writer, error) {
	var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
	escapeQuotes := func(s string) string {
		return quoteEscaper.Replace(s)
	}

	contentDispositionBuilder := strings.Builder{}
	contentDispositionBuilder.WriteString(`form-data; name="`)
	contentDispositionBuilder.WriteString(escapeQuotes(fieldname))
	contentDispositionBuilder.WriteString(`"`)

	if filename != nil {
		contentDispositionBuilder.WriteString(`; filename="`)
		contentDispositionBuilder.WriteString(escapeQuotes(*filename))
		contentDispositionBuilder.WriteString(`"`)
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", contentDispositionBuilder.String())
	log.Tracef("set field: %s, Content-Disposition: %s", fieldname, contentDispositionBuilder.String())

	if contentType != nil {
		h.Set("Content-Type", *contentType)
		log.Tracef("set field: %s, Content-Type: %s", fieldname, *contentType)
	} else {
		if filename != nil {
			h.Set("Content-Type", "application/octet-stream")
			log.Warnf("set field: %s, no Content-Type for file: %s, use application/octet-stream", fieldname, *filename)
		}
	}
	return b.w.CreatePart(h)
}

func (b *FormBodyBuilder) parseFormEntry(formEntryInput string) (ms [][2]string) {
	// 首先完全使用;分割
	for _, kv := range strings.Split(formEntryInput, ";") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		// 寻找第一个=，分割
		idx := strings.Index(kv, "=")
		if idx == -1 {
			log.Warn("invalid form entry: " + kv)
			continue
		}
		key := strings.TrimSpace(kv[:idx])
		value := strings.TrimSpace(kv[idx+1:])
		var m [2]string
		m[0], m[1] = key, value
		ms = append(ms, m)
	}
	return
}

func (b *FormBodyBuilder) Build(formEntries []string) (err error) {
	for _, F := range formEntries {
		if !strings.Contains(F, "=") {
			err = fmt.Errorf("invalid form entry: %s", F)
			log.Error(err)
			return
		}
		var (
			fieldname   string
			filename    *string
			contentType *string
			reader      io.ReadCloser
		)

		// 解析一个-F表单项，提取出其中的相关字段
		for i, pair := range b.parseFormEntry(F) {
			key, value := pair[0], pair[1]
			if i == 0 {
				// 内容逻辑
				fieldname = key
				if strings.HasPrefix(value, "@") {
					// 文件
					filename = new(string)
					*filename = value[1:]
					reader, err = os.Open(*filename)
					if err != nil {
						log.Errorf("open file: %s error: %s", *filename, err.Error())
						return
					}
				} else {
					// 普通内容
					reader = io.NopCloser(strings.NewReader(value))
				}
				continue
			}
			switch key {
			case "filename":
				filename = &value
			case "type":
				contentType = &value
			default:
				err = fmt.Errorf("invalid form entry: %s", F)
				return
			}
		}

		// 创建一个 part
		var fw io.Writer
		fw, err = b.createFormPart(fieldname, filename, contentType)
		if err != nil {
			log.Error("createFormPart error: ", err)
			return
		}

		// 向 part 中写入数据
		_, err = io.Copy(fw, bufio.NewReader(reader))
		if err != nil {
			log.Error("Copy error: ", err)
			return
		}

		// 关闭reader
		if err = reader.Close(); err != nil {
			log.Error("reader.Close error: ", err)
			return
		}

		log.Trace("CreateFormFile success")
	}

	// 关闭这个part的writer
	if err = b.w.Close(); err != nil {
		log.Error("w.Close error: ", err)
		return
	}
	return
}

func BuildFormData(req *http.Request, formEntries []string) {
	pipeReader, pipeWriter := io.Pipe()

	multipartWriter := multipart.NewWriter(pipeWriter)
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	req.Body = pipeReader

	// 后台逐步向pipeWriter中写入表单数据
	go func() {
		var err error
		defer pipeWriter.CloseWithError(err)
		defer multipartWriter.Close()
		if err = (&FormBodyBuilder{w: multipartWriter}).Build(formEntries); err != nil {
			log.Error("build & write multipart body error: ", err)
			return
		}
	}()
}
