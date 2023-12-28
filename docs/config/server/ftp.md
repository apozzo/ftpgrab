# FTP server configuration

!!! warning
    `ftp`,`sftp` and `http` are mutually exclusive

!!! example
    ```yaml
    server:
      ftp:
        host: test.rebex.net
        port: 21
        username: demo
        password: password
        sources:
          - /
        timeout: 5s
        disableUTF8: false
        disableEPSV: false
        disableMLSD: false
        escapeRegexpMeta: false
        tls: false
        explicittls: false
        insecureSkipVerify: false
        logTrace: false
    ```

## Reference

### `host`

FTP host IP or domain.

!!! example "Config file"
    ```yaml
    server:
      ftp:
        host: 127.0.0.1
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_HOST`

### `port`

FTP port. (default `21`)

!!! example "Config file"
    ```yaml
    server:
      ftp:
        port: 21
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_PORT`

### `username`

FTP username.

!!! example "Config file"
    ```yaml
    server:
      ftp:
        username: foo
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_USERNAME`

### `usernameFile`

Use content of secret file as FTP username if `username` not defined.

!!! example "Config file"
    ```yaml
    server:
      ftp:
        usernameFile: /run/secrets/username
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_USERNAMEFILE`

### `password`

FTP password.

!!! example "Config file"
    ```yaml
    server:
      ftp:
        password: bar
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_PASSWORD`

### `passwordFile`

Use content of secret file as FTP password if `password` not defined.

!!! example "Config file"
    ```yaml
    server:
      ftp:
        passwordFile: /run/secrets/password
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_PASSWORDFILE`

### `sources`

List of sources paths to grab from FTP server.

!!! example "Config file"
    ```yaml
    server:
      ftp:
        sources:
          - /path1
          - /path2/folder
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_SOURCES`

### `timeout`

Timeout for opening connections, sending control commands, and each read/write of data transfers. (default `5s`)

!!! example "Config file"
    ```yaml
    server:
      ftp:
        timeout: 5s
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_TIMEOUT`

### `disableUTF8`

Do not issue the `OPTS UTF8 ON` command (default `false`).

!!! example "Config file"
    ```yaml
    server:
      ftp:
        disableUTF8: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_DISABLEUTF8`

### `disableEPSV`

Disables EPSV in favour of PASV. This is useful in cases where EPSV connections neither complete nor downgrade to
PASV successfully by themselves, resulting in hung connections. (default `false`)

!!! example "Config file"
    ```yaml
    server:
      ftp:
        disableEPSV: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_DISABLEEPSV`

### `disableMLSD`

Disables MLSD can be useful for servers which advertise MLSD (e.g. some versions of Serv-U) but don't support it
properly. (default `false`)

!!! example "Config file"
    ```yaml
    server:
      ftp:
        disableMLSD: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_DISABLEMLSD`

### `escapeRegexpMeta`

Escapes all regular expression metacharacters in the source path. (default `false`)

!!! warning
    This setting is only useful for FTP servers that enforce global matching or
    if you don't want to use regular expressions when listing files. See [crazy-max/ftpgrab#49](https://github.com/crazy-max/ftpgrab/issues/49#issuecomment-489137115)
    for more information.

!!! example "Config file"
    ```yaml
    server:
      ftp:
        escapeRegexpMeta: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_ESCAPEREGEXPMETA`

### `explicittls`

Use explicitTls FTP over TLS. (default `false`)

!!! example "Config file"
    ```yaml
    server:
      ftp:
        explicittls: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_EXPLICITTLS`

### `tls`

Use implicit FTP over TLS. (default `false`)

!!! example "Config file"
    ```yaml
    server:
      ftp:
        tls: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_TLS`

### `insecureSkipVerify`

Controls whether a client verifies the server’s certificate chain and host name. (default `false`)

!!! example "Config file"
    ```yaml
    server:
      type: ftp
      ftp:
        insecureSkipVerify: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_INSECURESKIPVERIFY`

### `logTrace`

Enable low-level FTP log. Works only if global log level is debug. (default `false`)

!!! example "Config file"
    ```yaml
    server:
      type: ftp
      ftp:
        logTrace: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_FTP_LOGTRACE`
