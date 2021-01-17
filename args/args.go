package args

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	C "github.com/maskedeken/gost-plugin/constant"
)

type Options struct {
	LocalAddr  string
	LocalPort  uint
	RemoteAddr string
	RemotePort uint
	Hostname   string
	Mode       string
	Path       string
	ServerName string
	Server     bool
	Nocomp     bool
	Insecure   bool
	Cert       string
	Key        string
	Mux        uint
	LogLevel   uint
}

func (o *Options) GetLocalAddr() string {
	return fmt.Sprintf("%s:%d", fixAddr(o.LocalAddr), o.LocalPort)
}

func (o *Options) GetRemoteAddr() string {
	return fmt.Sprintf("%s:%d", fixAddr(o.RemoteAddr), o.RemotePort)
}

// Args maps a string key to a list of values. It is similar to url.Values.
type Args map[string][]string

// Get the first value associated with the given key. If there are any values
// associated with the key, the value return has the value and ok is set to
// true. If there are no values for the given key, value is "" and ok is false.
// If you need access to multiple values, use the map directly.
func (args Args) Get(key string) (value string, ok bool) {
	if args == nil {
		return "", false
	}
	vals, ok := args[key]
	if !ok || len(vals) == 0 {
		return "", false
	}
	return vals[0], true
}

// Append value to the list of values for key.
func (args Args) Add(key, value string) {
	args[key] = append(args[key], value)
}

// Return the index of the next unescaped byte in s that is in the term set, or
// else the length of the string if no terminators appear. Additionally return
// the unescaped string up to the returned index.
func indexUnescaped(s string, term []byte) (int, string, error) {
	var i int
	unesc := make([]byte, 0)
	for i = 0; i < len(s); i++ {
		b := s[i]
		// A terminator byte?
		if bytes.IndexByte(term, b) != -1 {
			break
		}
		if b == '\\' {
			i++
			if i >= len(s) {
				return 0, "", fmt.Errorf("nothing following final escape in %q", s)
			}
			b = s[i]
		}
		unesc = append(unesc, b)
	}
	return i, string(unesc), nil
}

func ApplyOptions(ctx context.Context, options *Options) (context.Context, error) {

	args, err := parseEnv()

	if err == nil {
		if c, b := args.Get("mode"); b {
			options.Mode = c
		}
		if _, b := args.Get("nocomp"); b {
			options.Nocomp = true
		}
		if _, b := args.Get("insecure"); b {
			options.Insecure = true
		}
		if c, b := args.Get("mux"); b {
			mux, err := strconv.ParseUint(c, 10, 16)
			if err != nil {
				return ctx, err
			}
			options.Mux = uint(mux)
		}
		if c, b := args.Get("host"); b {
			options.Hostname = c
		}
		if c, b := args.Get("path"); b {
			options.Path = c
		}
		if c, b := args.Get("cert"); b {
			options.Cert = c
		}
		if c, b := args.Get("key"); b {
			options.Key = c
		}
		if _, b := args.Get("server"); b {
			options.Server = true
		}
		if c, b := args.Get("serverName"); b {
			options.ServerName = c
		}
		if c, b := args.Get("localAddr"); b {
			if options.Server {
				options.RemoteAddr = c
			} else {
				options.LocalAddr = c
			}
		}
		if c, b := args.Get("localPort"); b {
			lport, err := strconv.ParseUint(c, 10, 32)
			if err != nil {
				return ctx, err
			}

			if options.Server {
				options.RemotePort = uint(lport)
			} else {
				options.LocalPort = uint(lport)
			}
		}
		if c, b := args.Get("remoteAddr"); b {
			if options.Server {
				options.LocalAddr = c
			} else {
				options.RemoteAddr = c
			}
		}
		if c, b := args.Get("remotePort"); b {
			rport, err := strconv.ParseUint(c, 10, 32)
			if err != nil {
				return ctx, err
			}

			if options.Server {
				options.LocalPort = uint(rport)
			} else {
				options.RemotePort = uint(rport)
			}
		}

	}

	ctx = context.WithValue(ctx, C.OPTIONS, options)
	return ctx, err
}

func parseEnv() (opts Args, err error) {
	opts = make(Args)
	ss_remote_host := os.Getenv("SS_REMOTE_HOST")
	ss_remote_port := os.Getenv("SS_REMOTE_PORT")
	ss_local_host := os.Getenv("SS_LOCAL_HOST")
	ss_local_port := os.Getenv("SS_LOCAL_PORT")
	if len(ss_remote_host) == 0 {
		return
	}
	if len(ss_remote_port) == 0 {
		return
	}
	if len(ss_local_host) == 0 {
		return
	}
	if len(ss_local_host) == 0 {
		return
	}

	opts.Add("remoteAddr", ss_remote_host)
	opts.Add("remotePort", ss_remote_port)
	opts.Add("localAddr", ss_local_host)
	opts.Add("localPort", ss_local_port)

	ss_plugin_options := os.Getenv("SS_PLUGIN_OPTIONS")
	if len(ss_plugin_options) > 0 {
		other_opts, err := parsePluginOptions(ss_plugin_options)
		if err != nil {
			return nil, err
		}
		for k, v := range other_opts {
			opts[k] = v
		}
	}
	return opts, nil
}

// Parse a name–value mapping as from SS_PLUGIN_OPTIONS.
//
// "<value> is a k=v string value with options that are to be passed to the
// transport. semicolons, equal signs and backslashes must be escaped
// with a backslash."
// Example: secret=nou;cache=/tmp/cache;secret=yes
func parsePluginOptions(s string) (opts Args, err error) {
	opts = make(Args)
	if len(s) == 0 {
		return
	}
	i := 0
	for {
		var key, value string
		var offset, begin int

		if i >= len(s) {
			break
		}
		begin = i
		// Read the key.
		offset, key, err = indexUnescaped(s[i:], []byte{'=', ';'})
		if err != nil {
			return
		}
		if len(key) == 0 {
			err = fmt.Errorf("empty key in %q", s[begin:i])
			return
		}
		i += offset
		// End of string or no equals sign?
		if i >= len(s) || s[i] != '=' {
			opts.Add(key, "1")
			// Skip the semicolon.
			i++
			continue
		}
		// Skip the equals sign.
		i++
		// Read the value.
		offset, value, err = indexUnescaped(s[i:], []byte{';'})
		if err != nil {
			return
		}
		i += offset
		opts.Add(key, value)
		// Skip the semicolon.
		i++
	}
	return opts, nil
}

// Escape backslashes and all the bytes that are in set.
func backslashEscape(s string, set []byte) string {
	var buf bytes.Buffer
	for _, b := range []byte(s) {
		if b == '\\' || bytes.IndexByte(set, b) != -1 {
			buf.WriteByte('\\')
		}
		buf.WriteByte(b)
	}
	return buf.String()
}

// Encode a name–value mapping so that it is suitable to go in the ARGS option
// of an SMETHOD line. The output is sorted by key. The "ARGS:" prefix is not
// added.
//
// "Equal signs and commas [and backslashes] must be escaped with a backslash."
func encodeSmethodArgs(args Args) string {
	if args == nil {
		return ""
	}

	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	escape := func(s string) string {
		return backslashEscape(s, []byte{'=', ','})
	}

	var pairs []string
	for _, key := range keys {
		for _, value := range args[key] {
			pairs = append(pairs, escape(key)+"="+escape(value))
		}
	}

	return strings.Join(pairs, ",")
}

func fixAddr(addr string) string {
	size := len(addr)
	if size == 0 {
		return addr
	}

	if strings.Contains(addr, ":") && addr[0] != '[' && addr[size-1] != ']' {
		return "[" + addr + "]"
	}

	return addr
}
