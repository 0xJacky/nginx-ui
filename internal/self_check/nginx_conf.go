package self_check

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/dumper"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

// CheckNginxConfIncludeSites checks if nginx.conf include sites-enabled
func CheckNginxConfIncludeSites() error {
	path := nginx.GetConfEntryPath()

	content, err := os.ReadFile(path)
	if err != nil {
		return ErrFailedToReadNginxConf
	}

	// parse nginx.conf
	p := parser.NewStringParser(string(content), parser.WithSkipValidDirectivesErr())
	c, err := p.Parse()
	if err != nil {
		return ErrParseNginxConf
	}

	// find http block
	for _, v := range c.Block.Directives {
		if v.GetName() == "http" {
			// find include sites-enabled
			for _, directive := range v.GetBlock().GetDirectives() {
				if directive.GetName() == "include" && len(directive.GetParameters()) > 0 &&
					strings.Contains(directive.GetParameters()[0].Value, "sites-enabled/*") {
					return nil
				}
			}
			return ErrNginxConfNotIncludeSitesEnabled
		}
	}

	return ErrNginxConfNoHttpBlock
}

// CheckNginxConfIncludeStreams checks if nginx.conf include streams-enabled
func CheckNginxConfIncludeStreams() error {
	path := nginx.GetConfEntryPath()

	content, err := os.ReadFile(path)
	if err != nil {
		return ErrFailedToReadNginxConf
	}

	// parse nginx.conf
	p := parser.NewStringParser(string(content), parser.WithSkipValidDirectivesErr())
	c, err := p.Parse()
	if err != nil {
		return ErrParseNginxConf
	}

	// find http block
	for _, v := range c.Block.Directives {
		if v.GetName() == "stream" {
			// find include sites-enabled
			for _, directive := range v.GetBlock().GetDirectives() {
				if directive.GetName() == "include" && len(directive.GetParameters()) > 0 &&
					strings.Contains(directive.GetParameters()[0].Value, "streams-enabled/*") {
					return nil
				}
			}
			return ErrNginxConfNotIncludeStreamEnabled
		}
	}

	return ErrNginxConfNoStreamBlock
}

// FixNginxConfIncludeSites attempts to fix nginx.conf include sites-enabled
func FixNginxConfIncludeSites() error {
	path := nginx.GetConfEntryPath()

	content, err := os.ReadFile(path)
	if err != nil {
		return ErrFailedToReadNginxConf
	}

	// create a backup file (+.bak.timestamp)
	backupPath := fmt.Sprintf("%s.bak.%d", path, time.Now().Unix())
	err = os.WriteFile(backupPath, content, 0644)
	if err != nil {
		return ErrFailedToCreateBackup
	}

	// parse nginx.conf
	p := parser.NewStringParser(string(content), parser.WithSkipValidDirectivesErr())
	c, err := p.Parse()
	if err != nil {
		return ErrParseNginxConf
	}

	// find http block
	for _, v := range c.Block.Directives {
		if v.GetName() == "http" {
			// add include sites-enabled/* to http block
			includeDirective := &config.Directive{
				Name:       "include",
				Parameters: []config.Parameter{{Value: resolvePath("sites-enabled/*")}},
			}

			realBlock := v.GetBlock().(*config.HTTP)
			realBlock.Directives = append(realBlock.Directives, includeDirective)

			// write to file
			return os.WriteFile(path, []byte(dumper.DumpBlock(c.Block, dumper.IndentedStyle)), 0644)
		}
	}

	// if no http block, append http block with include sites-enabled/*
	content = append(content, fmt.Appendf(nil, "\nhttp {\n\tinclude %s;\n}\n", resolvePath("sites-enabled/*"))...)
	return os.WriteFile(path, content, 0644)
}

// FixNginxConfIncludeStreams attempts to fix nginx.conf include streams-enabled
func FixNginxConfIncludeStreams() error {
	path := nginx.GetConfEntryPath()

	content, err := os.ReadFile(path)
	if err != nil {
		return ErrFailedToReadNginxConf
	}

	// create a backup file (+.bak.timestamp)
	backupPath := fmt.Sprintf("%s.bak.%d", path, time.Now().Unix())
	err = os.WriteFile(backupPath, content, 0644)
	if err != nil {
		return ErrFailedToCreateBackup
	}

	// parse nginx.conf
	p := parser.NewStringParser(string(content), parser.WithSkipValidDirectivesErr())
	c, err := p.Parse()
	if err != nil {
		return ErrParseNginxConf
	}

	// find stream block
	for _, v := range c.Block.Directives {
		if v.GetName() == "stream" {
			// add include streams-enabled/* to stream block
			includeDirective := &config.Directive{
				Name:       "include",
				Parameters: []config.Parameter{{Value: resolvePath("streams-enabled/*")}},
			}
			realBlock := v.GetBlock().(*config.Block)
			realBlock.Directives = append(realBlock.Directives, includeDirective)

			// write to file
			return os.WriteFile(path, []byte(dumper.DumpBlock(c.Block, dumper.IndentedStyle)), 0644)
		}
	}

	// if no stream block, append stream block with include streams-enabled/*
	content = append(content, fmt.Appendf(nil, "\nstream {\n\tinclude %s;\n}\n", resolvePath("streams-enabled/*"))...)
	return os.WriteFile(path, content, 0644)
}

// CheckNginxConfIncludeConfD checks if nginx.conf includes conf.d directory
func CheckNginxConfIncludeConfD() error {
	path := nginx.GetConfEntryPath()

	content, err := os.ReadFile(path)
	if err != nil {
		return ErrFailedToReadNginxConf
	}

	// parse nginx.conf
	p := parser.NewStringParser(string(content), parser.WithSkipValidDirectivesErr())
	c, err := p.Parse()
	if err != nil {
		return ErrParseNginxConf
	}

	// find http block
	for _, v := range c.Block.Directives {
		if v.GetName() == "http" {
			// find include conf.d
			for _, directive := range v.GetBlock().GetDirectives() {
				if directive.GetName() == "include" && len(directive.GetParameters()) > 0 &&
					strings.Contains(directive.GetParameters()[0].Value, "conf.d/*") {
					return nil
				}
			}
			return ErrNginxConfNotIncludeConfD
		}
	}

	return ErrNginxConfNoHttpBlock
}

// FixNginxConfIncludeConfD attempts to fix nginx.conf to include conf.d directory
func FixNginxConfIncludeConfD() error {
	path := nginx.GetConfEntryPath()

	content, err := os.ReadFile(path)
	if err != nil {
		return ErrFailedToReadNginxConf
	}

	// create a backup file (+.bak.timestamp)
	backupPath := fmt.Sprintf("%s.bak.%d", path, time.Now().Unix())
	err = os.WriteFile(backupPath, content, 0644)
	if err != nil {
		return ErrFailedToCreateBackup
	}

	// parse nginx.conf
	p := parser.NewStringParser(string(content), parser.WithSkipValidDirectivesErr())
	c, err := p.Parse()
	if err != nil {
		return ErrParseNginxConf
	}

	// find http block
	for _, v := range c.Block.Directives {
		if v.GetName() == "http" {
			// add include conf.d/*.conf to http block
			includeDirective := &config.Directive{
				Name:       "include",
				Parameters: []config.Parameter{{Value: resolvePath("conf.d/*")}},
			}

			realBlock := v.GetBlock().(*config.HTTP)
			realBlock.Directives = append(realBlock.Directives, includeDirective)

			// write to file
			return os.WriteFile(path, []byte(dumper.DumpBlock(c.Block, dumper.IndentedStyle)), 0644)
		}
	}

	// if no http block, append http block with include conf.d/*.conf
	content = append(content, fmt.Appendf(nil, "\nhttp {\n\tinclude %s;\n}\n", resolvePath("conf.d/*"))...)
	return os.WriteFile(path, content, 0644)
}
