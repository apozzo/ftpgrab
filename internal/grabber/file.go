package grabber

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/antonmedv/expr"
	"github.com/crazy-max/ftpgrab/v7/internal/server/ftp"
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

func (c *Client) ListFiles() []File {
	var files []File

	// Iterate sources
	for _, src := range c.ListExprSrc() {
		source := src
		log.Debug().Str("source", source).Msg("Listing files")

		// Check basedir
		dest := c.formatExprPath(c.config.Output)
		if source != "/" && *c.config.CreateBaseDir {
			dest = path.Join(dest, source)
		}

		files = append(files, c.readDir(source, source, dest, 0)...)
	}

	return files
}

func (c *Client) readDir(base string, srcdir string, destdir string, retry int) []File {
	var files []File

	log.Debug().Str("source", base).Msgf("Read directory %s, retry %d/%d", srcdir, retry, c.config.Retry)

	items, err := c.server.ReadDir(srcdir)
	if err != nil {

		retry++
		log.Error().Err(err).Str("source", base).Msgf("Cannot read directory %s, retry %d/%d", srcdir, retry, c.config.Retry)

		if retry == c.config.Retry {
			log.Error().Err(err).Str("source", base).Msgf("Cannot read directory %s", srcdir)
			return []File{}
		} else {
			// on relance une connexion pour le retry
			var err error

			// close client
			c.server.Close()

			// Server client
			if c.serverconfig.FTP != nil {
				c.server, err = ftp.New(c.serverconfig.FTP)
			} else if c.serverconfig.SFTP != nil {
				c.server, err = sftp.New(c.serverconfig.SFTP)
			} else {
				log.Error().Str("source", base).Msgf("No server defined, cannot read directory %s", srcdir)
				return []File{}
			}

			if err != nil {
				log.Error().Str("source", base).Msgf("Cannot connect to server, cannot read directory %s", srcdir)
				panic(err)

			} else {
				return c.readDir(base, srcdir, destdir, retry)
			}
		}
	}

	for _, item := range items {
		files = append(files, c.readFile(base, srcdir, destdir, item)...)
	}

	return files
}

func (c *Client) readFile(base string, srcdir string, destdir string, file os.FileInfo) []File {
	srcfile := path.Join(srcdir, file.Name())
	destfile := path.Join(destdir, file.Name())

	if file.IsDir() {
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
