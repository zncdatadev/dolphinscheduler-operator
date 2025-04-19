package common

import (
	"path"
	"strings"

	"github.com/zncdatadev/operator-go/pkg/config"
	"github.com/zncdatadev/operator-go/pkg/productlogging"

	loggingv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"

	"github.com/zncdatadev/operator-go/pkg/constants"
)

const logbackTemplate = `<configuration>
<appender name="CONSOLE" class="ch.qos.logback.core.ConsoleAppender">
<encoder>
  <pattern>{{ .ConsoleHandlerFormatter }}</pattern>
</encoder>
<filter class="ch.qos.logback.classic.filter.ThresholdFilter">
  <level>{{ .ConsoleHandlerLevel }}</level>
</filter>
</appender>

<appender name="FILE" class="ch.qos.logback.core.rolling.RollingFileAppender">
<File>{{ .RotatingFileHandlerFile }}</File>
<encoder class="ch.qos.logback.core.encoder.LayoutWrappingEncoder">
  <layout class="ch.qos.logback.classic.log4j.XMLLayout" />
</encoder>
<filter class="ch.qos.logback.classic.filter.ThresholdFilter">
  <level>{{ .RotatingFileHandlerLevel }}</level>
</filter>
<rollingPolicy class="ch.qos.logback.core.rolling.FixedWindowRollingPolicy">
  <minIndex>1</minIndex>
  <maxIndex>{{ .RotatingFileHandlerBackupCount }}</maxIndex>
  <FileNamePattern>{{ .RotatingFileHandlerFile }}.%i</FileNamePattern>
</rollingPolicy>
<triggeringPolicy class="ch.qos.logback.core.rolling.SizeBasedTriggeringPolicy">
  <MaxFileSize>{{ .RotatingFileHandlerMaxSizeInMiB }}MB</MaxFileSize>
</triggeringPolicy>
</appender>

{{- .Loggers }}

<root level="{{ .RootLogLevel }}">
  <appender-ref ref="CONSOLE" />
  <appender-ref ref="FILE" />
</root>
</configuration>
`

var _ productlogging.LoggingConfig = &LogbackConfig{}

// LogbackConfig is a struct that contains logback logging configuration
type LogbackConfig struct {
	productLogging *productlogging.ProductLogging
}

// Content implements the LoggingConfig interface
func (l *LogbackConfig) Content() (string, error) {
	values := productlogging.JavaLogTemplateValue(l, l.productLogging)

	p := config.TemplateParser{Template: l.Template(), Value: values}
	return p.Parse()
}

// LoggerFormatter implements the LoggingConfig interface
func (l *LogbackConfig) LoggerFormatter(name string, level string) string {
	return `<logger name="` + name + `" level="` + level + `" />`
}

// String implements the LoggingConfig interface
func (l *LogbackConfig) String() string {
	c, e := l.Content()
	if e != nil {
		panic(e)
	}
	return c
}

// Template implements the LoggingConfig interface
func (l *LogbackConfig) Template() string {
	return logbackTemplate
}

func NewConfigGenerator(
	loggingConfigSpec *loggingv1alpha1.LoggingConfigSpec,
	containerName string,
	logFileName string,
	logType productlogging.LogType,
	opts ...productlogging.ConfigGeneratorOptionFunc,
) (*LogbackConfig, error) {
	p := initializeProductLogging(containerName, logFileName, opts...)
	if loggingConfigSpec != nil {
		applyLoggingConfig(p, loggingConfigSpec)
	}
	return &LogbackConfig{productLogging: p}, nil
}

func initializeProductLogging(containerName string, logFileName string, opts ...productlogging.ConfigGeneratorOptionFunc) *productlogging.ProductLogging {
	opt := &productlogging.ConfigGeneratorOption{}
	for _, o := range opts {
		o(opt)
	}

	rotatingFileHandlerMaxBytes := productlogging.DefaultRotatingFileHandlerMaxBytes
	if opt.LogFileMaxBytes != nil {
		rotatingFileHandlerMaxBytes = *opt.LogFileMaxBytes
	}

	return &productlogging.ProductLogging{
		RootLogLevel:                   productlogging.DefaultLoggerLevel,
		ConsoleHandlerLevel:            productlogging.DefaultLoggerLevel,
		ConsoleHandlerFormatter:        *opt.ConsoleHandlerFormatter,
		RotatingFileHandlerLevel:       productlogging.DefaultLoggerLevel,
		RotatingFileHandlerFile:        path.Join(constants.KubedoopLogDir, strings.ToLower(containerName), logFileName),
		RotatingFileHandlerMaxBytes:    rotatingFileHandlerMaxBytes,
		RotatingFileHandlerBackupCount: 1,
		Loggers:                        make(map[string]loggingv1alpha1.LogLevelSpec),
	}
}

func applyLoggingConfig(p *productlogging.ProductLogging, loggingConfigSpec *loggingv1alpha1.LoggingConfigSpec) {
	applyHandlerLevels(p, loggingConfigSpec)
	applyLoggerLevels(p, loggingConfigSpec)
}

func applyHandlerLevels(p *productlogging.ProductLogging, loggingConfigSpec *loggingv1alpha1.LoggingConfigSpec) {
	if loggingConfigSpec.Console != nil && loggingConfigSpec.Console.Level != "" {
		p.ConsoleHandlerLevel = loggingConfigSpec.Console.Level
	}
	if loggingConfigSpec.File != nil && loggingConfigSpec.File.Level != "" {
		p.RotatingFileHandlerLevel = loggingConfigSpec.File.Level
	}
}

func applyLoggerLevels(p *productlogging.ProductLogging, loggingConfigSpec *loggingv1alpha1.LoggingConfigSpec) {
	if loggingConfigSpec.Loggers != nil {
		for name, level := range loggingConfigSpec.Loggers {
			if name == productlogging.RootLoggerName {
				p.RootLogLevel = level.Level
			} else {
				p.Loggers[name] = *level
			}
		}
	}
}
