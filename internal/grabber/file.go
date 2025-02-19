package grabber

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/antonmedv/expr"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/crazy-max/ftpgrab/v7/internal/server/ftp"
	"github.com/crazy-max/ftpgrab/v7/internal/server/http"
	"github.com/crazy-max/ftpgrab/v7/internal/server/sftp"
	"github.com/rs/zerolog/log"
)

// File represents a file to grab
type File struct {
	Base    string
	SrcDir  string
	DestDir string
	Info    os.FileInfo
}

func (c *Client) formatExprPath(pathExpression string) string {
	env := map[string]any{
		"day":   time.Now().Day(),
		"year":  time.Now().Year(),
		"month": int(time.Now().Month()),
	}

	//outresult, err := expr.Eval(`join(["/output/directory", string(year), string(month-1), string(day-2)], "/")`, env)
	outresult, err := expr.Eval(pathExpression, env)
	if err != nil {
		panic(err)
	}

	return fmt.Sprint(outresult)
}

func (c *Client) ListExprSrc() []string {
	var sources []string

	for _, src := range c.server.Common().Sources {
		sources = append(sources, c.formatExprPath(src))
	}

	return sources
}

func (c *Client) getExprDest() string {
	return c.formatExprPath(c.config.Output)
}

func (c *Client) ListFiles() []File {
	var files []File

	// Iterate sources
	for _, src := range c.ListExprSrc() {
		source := src
		log.Debug().Str("source", source).Msg("Listing files")

		// Check basedir
		dest := c.getExprDest()
		if source != "/" && *c.config.CreateBaseDir {
			if c.serverconfig.HTTP != nil {
				// for http delete file from source
				dest = strings.Replace(path.Join(dest, filepath.Dir(source)), "s3:/", "s3://", 1)

			} else {
				dest = path.Join(dest, source)
			}
		}

		files = append(files, c.readDir(source, source, dest, 0)...)
	}

	return files
}

func (c *Client) readDir(base string, srcdir string, destdir string, retry int) []File {
	var files []File

	log.Debug().Str("source", base).Msgf("Read directory %s, retry %d/%d", srcdir, retry, c.config.Retry)

	if c.threaddelay > 0 {
		time.Sleep(time.Millisecond * time.Duration(c.threaddelay))
	}

	items, err := c.server.ReadDir(srcdir)
	if err != nil {
		retry++
		log.Error().Err(err).Str("source", base).Msgf("Cannot read directory %s, retry %d/%d", srcdir, retry, c.config.Retry)

		if retry == c.config.Retry {
			log.Error().Err(err).Str("source", base).Msgf("Cannot read directory %s", srcdir)
			return []File{}
		}

		// on relance une connexion pour le retry
		var err error

		// close client
		c.server.Close()

		// Server client
		if c.serverconfig.FTP != nil {
			c.server, err = ftp.New(c.serverconfig.FTP)
		} else if c.serverconfig.SFTP != nil {
			c.server, err = sftp.New(c.serverconfig.SFTP)
		} else if c.serverconfig.HTTP != nil {
			c.server, err = http.New(c.serverconfig.HTTP)
		} else {
			log.Error().Str("source", base).Msgf("No server defined, cannot read directory %s", srcdir)
			return []File{}
		}

		if err != nil {
			log.Error().Str("source", base).Msgf("Cannot connect to server, cannot read directory %s", srcdir)
			panic(err)
			//return []File{}
		}
		return c.readDir(base, srcdir, destdir, retry)
	}

	for _, item := range items {
		if c.serverconfig.HTTP != nil {
			file := []File{
				{
					Base:    filepath.Dir(base),
					SrcDir:  filepath.Dir(srcdir),
					DestDir: destdir,
					Info:    item,
				}}
			files = append(files, file...)
		} else {
			files = append(files, c.readFile(base, srcdir, destdir, item)...)
		}
	}

	return files
}

func (c *Client) readFile(base string, srcdir string, destdir string, file os.FileInfo) []File {
	srcfile := path.Join(srcdir, file.Name())
	destfile := path.Join(destdir, file.Name())

	if file.IsDir() && *c.config.Recursive {
		return c.readDir(base, srcfile, destfile, 0)
	}

	return []File{
		{
			Base:    base,
			SrcDir:  srcdir,
			DestDir: destdir,
			Info:    file,
		},
	}
}

func moveFile(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func moveFileToS3(oldpath, newpath string) error {

	s3loc := regexp.MustCompile(`s3://([[:alnum:]\.\-_]+)/(.+)$`)

	if subm := s3loc.FindAllStringSubmatch(newpath, -1); subm != nil {
		for _, sub := range subm {

			cfg, err := config.LoadDefaultConfig(context.TODO())
			if err != nil {
				log.Error().Str("oldpath", oldpath).Str("newpath", newpath).Err(err).Msg("Cannot load AWS config")
				return err
			}

			// Create an Amazon S3 service client
			client := s3.NewFromConfig(cfg)

			f, err := os.Open(oldpath)
			if err != nil {
				log.Error().Str("oldpath", oldpath).Str("newpath", newpath).Err(err).Msg("Error opening file to copy to S3")
				return err
			}

			// TODO: add md5sum to request for security
			output, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket: aws.String(sub[1]),
				Key:    aws.String(sub[2]),
				Body:   f,
			})

			defer f.Close()

			if output == nil && err != nil {
				log.Error().Str("oldpath", oldpath).Str("newpath", newpath).Err(err).Msg("Error moving file to S3")
				return err
			}

			log.Debug().Str("oldpath", oldpath).Str("newpath", newpath).Msgf("Successfully moved file to S3 from %s to %s", oldpath, newpath)
		}
		return nil
	}

	// no submatch !
	return errors.New("Cannot decode url " + newpath)
}

func statFileS3(filepath string) (fs.FileInfo, error) {

	s3loc := regexp.MustCompile(`s3://([[:alnum:]\.\-_]+)/(.+)$`)

	if subm := s3loc.FindAllStringSubmatch(filepath, -1); subm != nil {
		for _, sub := range subm {

			cfg, err := config.LoadDefaultConfig(context.TODO())
			if err != nil {
				log.Error().Str("filepath", filepath).Err(err).Msg("Cannot load AWS config")
				return nil, err
			}

			// Create an Amazon S3 service client
			client := s3.NewFromConfig(cfg)

			// TODO: add md5sum to request for security
			output, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
				Bucket: aws.String(sub[1]),
				Key:    aws.String(sub[2]),
			})

			if output == nil && err != nil {
				log.Error().Str("filepath", filepath).Err(err).Msg("Error head object from S3")
				return nil, err
			}

			log.Debug().Str("filepath", filepath).Msgf("Successfully stat file from S3 for file %s", filepath)

			return &fileInfo{
				name:    filepath,
				size:    *output.ContentLength,
				mode:    fs.FileMode(0777),
				modTime: *output.LastModified,
				isDir:   false,
				sys:     nil,
			}, nil
		}
		return nil, nil
	}

	// no submatch !
	return nil, errors.New("Cannot decode url " + filepath)
}
