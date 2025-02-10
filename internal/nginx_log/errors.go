package nginx_log

import "github.com/uozi-tech/cosy"

var (
	e                                      = cosy.NewErrorScope("nginx_log")
	ErrLogPathIsNotUnderTheLogDirWhiteList = e.New(50001, "the log path is not under the paths in settings.NginxSettings.LogDirWhiteList")
	ErrServerIdxOutOfRange                 = e.New(50002, "serverIdx out of range")
	ErrDirectiveIdxOutOfRange              = e.New(50003, "directiveIdx out of range")
	ErrLogDirective                        = e.New(50004, "directive.Params neither access_log nor error_log")
	ErrDirectiveParamsIsEmpty              = e.New(50005, "directive params is empty")
	ErrErrorLogPathIsEmpty                 = e.New(50006, "settings.NginxLogSettings.ErrorLogPath is empty, refer to https://nginxui.com/guide/config-nginx.html for more information")
	ErrAccessLogPathIsEmpty                = e.New(50007, "settings.NginxLogSettings.AccessLogPath is empty, refer to https://nginxui.com/guide/config-nginx.html for more information")
)
