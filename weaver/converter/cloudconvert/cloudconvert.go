package cloudconvert

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/arachnys/athenapdf/weaver/converter"
	"github.com/satori/go.uuid"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type CloudConvert struct {
	converter.UploadConversion
	Client
}

type Client struct {
	BaseURL string
	APIKey  string
}

type Process struct {
	ID      string `json:"id,omitempty"`
	URL     string `json:"url"`
	Expires string `json:"expires,omitempty"`
	MaxTime int    `json:"maxtime,omitempty"`
	Minutes int    `json:"minutes,omitempty"`
}

type S3 struct {
	AccessKey    string `json:"accesskeyid"`
	AccessSecret string `json:"secretaccesskey"`
	Bucket       string `json:"bucket"`
	Path         string `json:"path"`
	ACL          string `json:"acl"`
}

type Output struct {
	S3 `json:"s3"`
}

type Conversion struct {
	Input        string `json:"input"`
	File         string `json:"file"`
	Filename     string `json:"filename"`
	OutputFormat string `json:"outputformat"`
	Wait         bool   `json:"wait"`
	Download     string `json:"download,omitempty"`
	*Output      `json:"output,omitempty"`
}

func (c Client) NewProcess(inputFormat, ouputFormat string) (Process, error) {
	process := Process{}
	res, err := http.PostForm(
		c.BaseURL+"/process",
		url.Values{
			"apikey":       {c.APIKey},
			"inputformat":  {inputFormat},
			"outputformat": {ouputFormat},
		},
	)
	if err != nil {
		return process, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		var data map[string]interface{}
		if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
			return process, err
		}
		return process, errors.New(fmt.Sprintf("[CloudConvert] did not receive HTTP 200, response: %+v\n", data))
	}

	err = json.NewDecoder(res.Body).Decode(&process)
	if err == nil && strings.HasPrefix(process.URL, "//") {
		process.URL = "https:" + process.URL
	}

	return process, err
}

func (p Process) StartConversion(c Conversion) ([]byte, error) {
	b, err := json.Marshal(&c)
	if err != nil {
		return nil, err
	}
	res, err := http.Post(p.URL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		var data map[string]interface{}
		if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
			return nil, err
		}
		return nil, errors.New(fmt.Sprintf("[CloudConvert] did not receive HTTP 200, response: %+v\n", data))
	}

	if c.Download == "inline" {
		o, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		return o, nil
	}

	return nil, nil
}

func (c CloudConvert) Convert(done <-chan struct{}) ([]byte, error) {
	log.Printf("[CloudConvert] converting to PDF: %s\n", c.Path)

	p, err := c.Client.NewProcess("html", "pdf")
	if err != nil {
		return nil, err
	}

	conv := Conversion{
		Input:        "download",
		File:         c.Path,
		Filename:     c.AWSS3.S3Key + ".html",
		OutputFormat: "pdf",
		Wait:         true,
	}
	if c.AWSS3.S3Bucket == "" || c.AWSS3.S3Key == "" {
		conv.Download = "inline"
		conv.Filename = uuid.NewV4().String() + ".html"
	} else {
		conv.Output = &Output{
			S3{
				c.AWSS3.AccessKey,
				c.AWSS3.AccessSecret,
				c.AWSS3.S3Bucket,
				c.AWSS3.S3Key,
				"public-read",
			},
		}
		log.Printf("[CloudConvert] uploading conversion to S3: %s\n", c.AWSS3.S3Key)
	}

	b, err := p.StartConversion(conv)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (c CloudConvert) Upload(b []byte) (bool, error) {
	if c.AWSS3.S3Bucket == "" || c.AWSS3.S3Key == "" {
		return false, nil
	}
	return true, nil
}