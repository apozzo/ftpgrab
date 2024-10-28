package http

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/araddon/dateparse"
	"github.com/cavaliergopher/grab/v3"
	"github.com/crazy-max/ftpgrab/v7/internal/config"
	"github.com/crazy-max/ftpgrab/v7/internal/server"
	"github.com/rs/zerolog/log"
)

// Client represents an active http object
type Client struct {
	*server.Client
	config *config.ServerHTTP
	http   *grab.Client
}

// New creates new http instance
func New(config *config.ServerHTTP) (*server.Client, error) {
	var err error
	var client = &Client{config: config}

	/////////////////////////////////////////////////////////////////////////////////////
	// TODO : Basic authentication
	//
	// if len(config.Password) > 0 || len(config.PasswordFile) > 0 {
	// 	password, err := utl.GetSecret(config.Password, config.PasswordFile)
	// 	if err != nil {
	// 		log.Warn().Err(err).Msg("Cannot retrieve password secret for sftp server")
	// 	}
	// }

	// username, err := utl.GetSecret(config.Username, config.UsernameFile)
	// if err != nil {
	// 	log.Warn().Err(err).Msg("Cannot retrieve username secret for sftp server")
	// }
	//
	/////////////////////////////////////////////////////////////////////////////////////

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			// Set InsecureSkipVerify to skip the default validation we are
			// replacing. This will not disable VerifyConnection.
			InsecureSkipVerify: *config.InsecureSkipVerify,
		},
		TLSHandshakeTimeout: *config.Timeout,

		DisableKeepAlives:  *config.DisableKeepAlives,
		DisableCompression: *config.DisableCompression,

		// MaxIdleConns int
		// MaxIdleConnsPerHost int
		// MaxConnsPerHost int

		// IdleConnTimeout time.Duration
		ResponseHeaderTimeout: *config.Timeout,
	}

	client.http = grab.NewClient()
	client.http.HTTPClient = &http.Client{
		Transport: tr,
	}

	return &server.Client{Handler: client}, err
}

// Common return common configuration
func (c *Client) Common() config.ServerCommon {
	return config.ServerCommon{
		Host:    c.config.Host,
		Port:    c.config.Port,
		Sources: c.config.Sources,
	}
}

// for http urls it heads the url to retrieve size and date
func (c *Client) HeadUrl(path string, urlStr string) ([]os.FileInfo, error) {

	log.Debug().Msgf("HTTP Head url %s ...", urlStr)

	// prepare HEAD request
	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err != nil {
		log.Debug().Str("path", path).Msgf("Cannot create grab head request for url %s", urlStr)
		return nil, err
	}

	// send the request and get the response
	resp, err := c.http.HTTPClient.Do(req)
	if err != nil {
		log.Debug().Str("path", path).Err(err).Msgf("Error sending request to HEAD url %s !", urlStr)
		return nil, err
	}
	defer resp.Body.Close()

	// check if the response was a redirect
	if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		redirectURL, err := resp.Location()
		if err != nil {
			log.Debug().Str("path", path).Err(err).Msgf("Error getting redirect location for url %s !", urlStr)
			return nil, err
		}

		// create a new GET request to follow the redirect
		req.URL = redirectURL
		resp, err = c.http.HTTPClient.Do(req)
		if err != nil {
			log.Debug().Str("path", path).Err(err).Msgf("Error sending redirect request %s to HEAD url %s !", redirectURL, urlStr)
			return nil, err
		}
		defer resp.Body.Close()
	}

	// decode time
	var resptime time.Time

	if resp.Header.Get("last-modified") != "" {
		// header last-modified
		resptime, err = dateparse.ParseLocal(resp.Header.Get("last-modified"))
	} else if resp.Header.Get("date") != "" {
		// header date
		resptime, err = dateparse.ParseLocal(resp.Header.Get("date"))
	} else {
		// current date
		resptime = time.Now()
	}

	if err != nil {
		log.Debug().Str("path", path).Err(err).Msgf("Error decoding date for url %s !", urlStr)
		return nil, err
	}

	// log.Debug().Msgf("Decoded date for url %s : %v ", urlStr, resptime)

	var entries []os.FileInfo

	var mode os.FileMode

	fileInfo := &fileInfo{
		name:  filepath.Base(path),
		mode:  mode,
		mtime: resptime,
		size:  int64(resp.ContentLength),
	}

	entries = append(entries, fileInfo)

	return entries, nil

}

// ReadDir fetches the contents of a directory, returning a list of os.FileInfo's
// for http urls it heads the url to retrieve size and date
func (c *Client) ReadDir(path string) ([]os.FileInfo, error) {
	var urlStr string
	urlStr = "http"
	if *c.config.TLS {
		urlStr += "s"
	}

	// prepare url
	urlStr += "://" + c.config.Host + ":" + strconv.Itoa(c.config.Port) + path

	if *c.config.AutoIndex {

		urlStrAutoIndex := urlStr + "?F=0"

		log.Debug().Msgf("HTTP Get url %s ...", urlStrAutoIndex)

		// prepare GET request
		req, err := http.NewRequest("GET", urlStrAutoIndex, nil)
		if err != nil {
			log.Debug().Str("path", path).Msgf("Cannot create grab get request for url %s", urlStrAutoIndex)
			return nil, err
		}

		// send the request and get the response
		resp, err := c.http.HTTPClient.Do(req)
		if err != nil {
			log.Debug().Str("path", path).Err(err).Msgf("Error sending request to GET url %s !", urlStrAutoIndex)
			return nil, err
		}
		defer resp.Body.Close()

		// read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Debug().Str("path", path).Err(err).Msgf("Error reading Body to url %s !", urlStr)
			return nil, err
		}

		bodyBuffer := bytes.NewBuffer(body)
		reHref := regexp.MustCompile(`href="([^"]+)"`)
		relocalUrl := regexp.MustCompile(`^(?:\./)?(?:[[:alnum:]\.\-_])+$`)
		var entries []os.FileInfo

		for {
			line, err := bodyBuffer.ReadString('>') // No encoding ??
			if line != "" {
				if subm := reHref.FindAllStringSubmatch(line, -1); subm != nil {
					for _, sub := range subm {
						if relocalUrl.MatchString(sub[1]) {
							log.Debug().Msgf("Found href %s", sub[1])

							entry, err := c.HeadUrl(path+sub[1], urlStr+sub[1])
							if err == nil {
								entries = append(entries, entry...)
							}
						}
					}
				}
			}

			if err == io.EOF {
				break
			} else if err != nil {
				log.Debug().Str("path", path).Err(err).Msgf("Error reading Body to url %s !", urlStr)
				return nil, err
			}
		}

		return entries, nil

	} else {
		// autoindex is disabled for this server
		return c.HeadUrl(path, urlStr)
	}
}

// Retrieve file "path" from server and write bytes to "dest".
func (c *Client) Retrieve(path string, dest io.Writer) error {
	var urlStr string
	urlStr = "http"
	if *c.config.TLS {
		urlStr += "s"
	}

	// prepare url
	urlStr += "://" + c.config.Host + ":" + strconv.Itoa(c.config.Port) + path

	tempfile, err := os.CreateTemp("", "*")
	if err != nil {
		log.Debug().Str("path", path).Err(err).Msgf("Cannot create temporary file for url %s !", urlStr)
		return err
	}

	log.Debug().Msgf("HTTP Retrieve url %s ...", urlStr)

	req, err := grab.NewRequest(tempfile.Name(), urlStr)
	if err != nil {
		log.Debug().Str("path", path).Err(err).Msgf("Cannot create grab request for url %s !", urlStr)
		return err
	}

	resp := c.http.Do(req)
	if err := resp.Err(); err != nil {
		log.Debug().Str("path", path).Err(err).Msgf("Error exec grab request for url %s !", urlStr)
		return err
	}

	if _, err := io.Copy(dest, tempfile); err != nil {
		log.Debug().Str("path", path).Str("temfile", tempfile.Name()).Err(err).Msgf("Error copy temp file to destination for url %s !", urlStr)
		return err
	}

	defer os.Remove(tempfile.Name())

	return nil
}

// Close closes http connection
func (c *Client) Close() error {
	// http client in grab doesn't have close method
	// nothing to do ?
	return nil
}
