package grabber

import (
	"fmt"
	"sync"
	"sync/atomic"
	"os"
	"path"
	"time"

	"github.com/crazy-max/ftpgrab/v7/internal/config"
	"github.com/crazy-max/ftpgrab/v7/internal/db"
	"github.com/crazy-max/ftpgrab/v7/internal/journal"
	"github.com/crazy-max/ftpgrab/v7/internal/server"
	"github.com/crazy-max/ftpgrab/v7/internal/server/ftp"
	"github.com/crazy-max/ftpgrab/v7/internal/server/sftp"
	"github.com/crazy-max/ftpgrab/v7/pkg/utl"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Client represents an active grabber object
type Client struct {
	config  *config.Download
	db      *db.Client
	dbConfig *config.Db
	server  *server.Client
	serverconfig  *config.Server
	tempdir string
}

// New creates new grabber instance
func New(dlConfig *config.Download, dbConfig *config.Db, dbCli *db.Client, serverConfig *config.Server) (*Client, error) {
	var dbCliLocal *db.Client
	var serverCli *server.Client
	var err error

	if dbCli != nil {
		log.Debug().Msg("Using alreading opened DB connection")
		dbCliLocal = dbCli
	} else {
		// DB client
		log.Debug().Msg("Opening new DB connection")
		if dbCliLocal, err = db.New(dbConfig); err != nil {
			return nil, errors.Wrap(err, "Cannot open database")
		}
	}

	// Server client
	if serverConfig.FTP != nil {
		serverCli, err = ftp.New(serverConfig.FTP)
	} else if serverConfig.SFTP != nil {
		serverCli, err = sftp.New(serverConfig.SFTP)
	} else {
		return nil, errors.New("No server defined")
	}
	if err != nil {
		return nil, errors.Wrap(err, "Cannot connect to server")
	}

	// Temp dir to download files
	tempdir, err := os.MkdirTemp("", ".ftpgrab.*")
	if err != nil {
		return nil, errors.Wrap(err, "Cannot create temp dir")
	}

	return &Client{
		config:  dlConfig,
		db:      dbCliLocal,
		dbConfig: dbConfig,
		server:  serverCli,
		serverconfig: serverConfig,
		tempdir: tempdir,
	}, nil
}

func (c *Client) Grab(files []File, concurrency uint32) journal.Journal {
	jnl := journal.New()
	jnl.ServerHost = c.server.Common().Host

	log.Debug().Msg("Closing main connexion to Server ...")
	if err := c.server.Close(); err != nil {
		log.Warn().Err(err).Msg("Cannot close server connection")
	}
	log.Debug().Msg("Main connection to Server closed.")

	log.Info().Msgf("Using %d concurrency for downloading.", concurrency)

	var ops uint32 = 0
	var wg sync.WaitGroup

	for _, file := range files {
		
		for atomic.LoadUint32(&ops)>= concurrency {
			log.Info().Msgf("Waiting 1 second for a thread to finish, nb threads %d", atomic.LoadUint32(&ops))
			time.Sleep(1 * time.Second)
		}

		log.Debug().Msgf("Starting a new download thread for file %s, nb threads %d", path.Join(file.SrcDir, file.Info.Name()), atomic.AddUint32(&ops,1))
		wg.Add (1)

		go func(fileToDownload File) {

			defer wg.Done()
			
			var threadcli  *Client
			var err error

			if threadcli, err = New(c.config, c.dbConfig, c.db, c.serverconfig); err != nil {
				log.Warn().Err(err).Msg("Cannot create grabber")
			} else {
				defer threadcli.CloseWithoutDB()
				if entry := threadcli.download(fileToDownload, 0); entry != nil {
					jnl.Add(*entry)
				}
			}

			atomic.CompareAndSwapUint32(&ops, atomic.LoadUint32(&ops), atomic.LoadUint32(&ops)-1)
		}(file)		
	}

	log.Debug().Msgf("Queue is empty, remaining threads %d", atomic.LoadUint32(&ops))

	wg.Wait()

	log.Debug().Msgf("All threads finished, remaining threads %d in counter ( :-( if not zero )", atomic.LoadUint32(&ops))

	return jnl.Journal
}

func (c *Client) download(file File, retry int) *journal.Entry {
	srcpath := path.Join(file.SrcDir, file.Info.Name())
	destpath := path.Join(file.DestDir, file.Info.Name())

	entry := &journal.Entry{
		File:   srcpath,
		Status: c.getStatus(file),
	}

	sublogger := log.With().
		Str("src", entry.File).
		Str("dest", file.DestDir).
		Str("size", units.HumanSize(float64(file.Info.Size()))).
		Logger()

	if entry.Status == journal.EntryStatusAlreadyDl && !c.db.HasHash(file.Base, file.SrcDir, file.Info) {
		if err := c.db.PutHash(file.Base, file.SrcDir, file.Info); err != nil {
			sublogger.Error().Err(err).Msg("Cannot add hash into db")
			entry.Level = journal.EntryLevelWarning
			entry.Text = fmt.Sprintf("Already downloaded but cannot add hash into db: %v", err)
			return entry
		}
	}

	if entry.Status.IsSkipped() {
		if !*c.config.HideSkipped {
			sublogger.Warn().Msgf("Skipped (%s)", entry.Status)
			entry.Level = journal.EntryLevelSkip
			return entry
		}
		return nil
	}

	retrieveStart := time.Now()

	destfolder := path.Dir(destpath)
	if err := os.MkdirAll(destfolder, os.ModePerm); err != nil {
		sublogger.Error().Err(err).Msg("Cannot create destination dir")
		entry.Level = journal.EntryLevelError
		entry.Text = fmt.Sprintf("Cannot create destination dir: %v", err)
		return entry
	}
	if err := c.fixPerms(destfolder); err != nil {
		sublogger.Warn().Err(err).Msg("Cannot fix parent folder permissions")
	}

	destfile, err := c.createFile(destpath)
	if err != nil {
		sublogger.Error().Err(err).Msg("Cannot create destination file")
		entry.Level = journal.EntryLevelError
		entry.Text = fmt.Sprintf("Cannot create destination file: %v", err)
		return entry
	}
	defer destfile.Close()

	err = c.server.Retrieve(srcpath, destfile)
	if err != nil {
		retry++
		sublogger.Error().Err(err).Msgf("Error downloading, retry %d/%d", retry, c.config.Retry)
		if retry == c.config.Retry {
			sublogger.Error().Err(err).Msg("Cannot download file")
			entry.Level = journal.EntryLevelError
			entry.Text = fmt.Sprintf("Cannot download file: %v", err)
		} else {
			// on relance une connexion pour le retry
			var threadcli  *Client
			var err error

			if threadcli, err = New(c.config, c.dbConfig, c.db, c.serverconfig); err != nil {
				log.Warn().Err(err).Msg("Cannot create grabber")
			} else {
				defer threadcli.CloseWithoutDB()
				return c.download(file, retry)
			}



		}
	} else {
		if err = destfile.Close(); err != nil {
			sublogger.Error().Err(err).Msg("Cannot close destination file")
			entry.Level = journal.EntryLevelError
			entry.Text = fmt.Sprintf("Cannot close destination file: %v", err)
			return entry
		}

		if *c.config.TempFirst {
			log.Debug().
				Str("tempfile", destfile.Name()).
				Str("destfile", destpath).
				Msgf("Move temp file")
			err := moveFile(destfile.Name(), destpath)
			if err != nil {
				sublogger.Error().Err(err).Msg("Cannot move file")
				entry.Level = journal.EntryLevelError
				entry.Text = fmt.Sprintf("Cannot move file: %v", err)
				return entry
			}
		}

		sublogger.Info().
			Str("duration", time.Since(retrieveStart).Round(time.Millisecond).String()).
			Msg("File successfully downloaded")

		entry.Level = journal.EntryLevelSuccess
		entry.Text = fmt.Sprintf("%s successfully downloaded in %s",
			units.HumanSize(float64(file.Info.Size())),
			time.Since(retrieveStart).Round(time.Millisecond).String(),
		)
		if err := c.fixPerms(destpath); err != nil {
			sublogger.Warn().Err(err).Msg("Cannot fix file permissions")
		}
		if err := c.db.PutHash(file.Base, file.SrcDir, file.Info); err != nil {
			sublogger.Error().Err(err).Msg("Cannot add hash into db")
			entry.Level = journal.EntryLevelWarning
			entry.Text = fmt.Sprintf("Successfully downloaded but cannot add hash into db: %v", err)
		}
		if err = os.Chtimes(destpath, file.Info.ModTime(), file.Info.ModTime()); err != nil {
			sublogger.Warn().Err(err).Msg("Cannot change modtime of destination file")
		}
	}

	return entry
}

func (c *Client) createFile(filename string) (*os.File, error) {
	if *c.config.TempFirst {
		tempfile, err := os.CreateTemp(c.tempdir, path.Base(filename))
		if err != nil {
			return nil, err
		}
		return tempfile, nil
	}

	destfile, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return destfile, nil
}

func (c *Client) getStatus(file File) journal.EntryStatus {
	if !c.isIncluded(file) {
		return journal.EntryStatusNotIncluded
	} else if c.isExcluded(file) {
		return journal.EntryStatusExcluded
	} else if file.Info.ModTime().Before(c.config.SinceTime) {
		return journal.EntryStatusOutdated
	} else if destfile, err := os.Stat(path.Join(file.DestDir, file.Info.Name())); err == nil {
		if destfile.Size() == file.Info.Size() {
			return journal.EntryStatusAlreadyDl
		}
		return journal.EntryStatusSizeDiff
	} else if c.db.HasHash(file.Base, file.SrcDir, file.Info) {
		return journal.EntryStatusHashExists
	}
	return journal.EntryStatusNeverDl
}

func (c *Client) isIncluded(file File) bool {
	if len(c.config.Include) == 0 {
		return true
	}
	for _, include := range c.config.Include {
		if utl.MatchString(include, file.Info.Name()) {
			return true
		}
	}
	return false
}

func (c *Client) isExcluded(file File) bool {
	if len(c.config.Exclude) == 0 {
		return false
	}
	for _, exclude := range c.config.Exclude {
		if utl.MatchString(exclude, file.Info.Name()) {
			return true
		}
	}
	return false
}

// Close closes grabber
func (c *Client) CloseAll() {
	log.Debug().Msg("Closing all connections")
	if err := c.db.Close(); err != nil {
		log.Warn().Err(err).Msg("Cannot close database")
	}
	if err := c.server.Close(); err != nil {
		log.Warn().Err(err).Msg("Cannot close server connection")
	}
	if err := os.RemoveAll(c.tempdir); err != nil {
		log.Warn().Err(err).Msg("Cannot remove temp folder")
	}
}

func (c *Client) CloseWithoutDB() {
	log.Debug().Msg("Closing all connections but DB")
	if err := c.server.Close(); err != nil {
		log.Warn().Err(err).Msg("Cannot close server connection")
	}
	if err := os.RemoveAll(c.tempdir); err != nil {
		log.Warn().Err(err).Msg("Cannot remove temp folder")
	}
}

func (c *Client) CloseDB() {
	log.Debug().Msg("Closing DB connection")
	if err := c.db.Close(); err != nil {
		log.Warn().Err(err).Msg("Cannot close database")
	}
}
