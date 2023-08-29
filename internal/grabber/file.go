package grabber

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/antonmedv/expr"
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
	for _, src := range c.server.Common().Sources {
		source := c.formatExprPath(src)
		log.Debug().Str("source", source).Msg("Listing files")

		// Check basedir
		dest := c.formatExprPath(c.config.Output)
		if source != "/" && *c.config.CreateBaseDir {
			dest = path.Join(dest, source)
		}

		files = append(files, c.readDir(source, source, dest)...)
	}

	return files
}

func (c *Client) readDir(base string, srcdir string, destdir string) []File {
	var files []File

	items, err := c.server.ReadDir(srcdir)
	if err != nil {
		log.Error().Err(err).Str("source", base).Msgf("Cannot read directory %s", srcdir)
		return []File{}
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
		return c.readDir(base, srcfile, destfile)
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
