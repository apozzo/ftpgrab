package config

import (
	"time"

	"github.com/crazy-max/ftpgrab/v7/pkg/utl"
)

// ServerFTP holds ftp server configuration
type ServerHTTP struct {
	Host               string         `yaml:"host,omitempty" json:"host,omitempty" validate:"required"`
	Port               int            `yaml:"port,omitempty" json:"port,omitempty" validate:"required,min=1"`
	Username           string         `yaml:"username,omitempty" json:"username,omitempty"`
	UsernameFile       string         `yaml:"usernameFile,omitempty" json:"usernameFile,omitempty" validate:"omitempty,file"`
	Password           string         `yaml:"password,omitempty" json:"password,omitempty"`
	PasswordFile       string         `yaml:"passwordFile,omitempty" json:"passwordFile,omitempty" validate:"omitempty,file"`
	Sources            []string       `yaml:"sources,omitempty" json:"sources,omitempty"`
	Timeout            *time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	DisableKeepAlives  *bool          `yaml:"disablekeepalives,omitempty" json:"disablekeepalives,omitempty"`
	DisableCompression *bool          `yaml:"disablecompression,omitempty" json:"disablecompression,omitempty"`
	TLS                *bool          `yaml:"tls,omitempty" json:"tls,omitempty"`
	InsecureSkipVerify *bool          `yaml:"insecureSkipVerify,omitempty" json:"insecureSkipVerify,omitempty"`
	LogTrace           *bool          `yaml:"logTrace,omitempty" json:"logTrace,omitempty"`
	Proxy              string         `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	ProxyUsername      string         `yaml:"proxyUsername,omitempty" json:"proxyUsername,omitempty"`
	ProxyPassword      string         `yaml:"proxyPassword,omitempty" json:"proxyPassword,omitempty"`
}

// GetDefaults gets the default values
func (s *ServerHTTP) GetDefaults() *ServerHTTP {
	n := &ServerHTTP{}
	n.SetDefaults()
	return n
}

// SetDefaults sets the default values
func (s *ServerHTTP) SetDefaults() {
	s.Port = 80
	s.Sources = []string{}
	s.Timeout = utl.NewDuration(5 * time.Second)
	s.DisableKeepAlives = utl.NewFalse()
	s.DisableCompression = utl.NewFalse()
	s.TLS = utl.NewFalse()
	s.InsecureSkipVerify = utl.NewFalse()
	s.LogTrace = utl.NewFalse()
}
