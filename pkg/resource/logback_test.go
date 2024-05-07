package resource

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
	"text/template"
)

const origin = `<?xml version="1.0" encoding="UTF-8"?>
<!--
  ~ Licensed to the Apache Software Foundation (ASF) under one or more
  ~ contributor license agreements.  See the NOTICE file distributed with
  ~ this work for additional information regarding copyright ownership.
  ~ The ASF licenses this file to You under the Apache License, Version 2.0
  ~ (the "License"); you may not use this file except in compliance with
  ~ the License.  You may obtain a copy of the License at
  ~
  ~     http://www.apache.org/licenses/LICENSE-2.0
  ~
  ~ Unless required by applicable law or agreed to in writing, software
  ~ distributed under the License is distributed on an "AS IS" BASIS,
  ~ WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  ~ See the License for the specific language governing permissions and
  ~ limitations under the License.
  -->

<configuration scan="true" scanPeriod="120 seconds">
    <property name="log.base" value="logs"/>
    <property scope="context" name="log.base.ctx" value="${log.base}" />

    <appender name="STDOUT" class="ch.qos.logback.core.ConsoleAppender">
    {{- if .console}}
		<filter class="ch.qos.logback.classic.filter.ThresholdFilter">
			<level>{{.console}}</level>
		</filter>
	{{- end}}
        <encoder>
            <pattern>
                [WI-%X{workflowInstanceId:-0}][TI-%X{taskInstanceId:-0}] - [%level] %date{yyyy-MM-dd HH:mm:ss.SSS Z} %logger{10}:[%line] - %msg%n
            </pattern>
            <charset>UTF-8</charset>
        </encoder>
    </appender>

    <conversionRule conversionWord="message"
                    converterClass="org.apache.dolphinscheduler.common.log.SensitiveDataConverter"/>
    <appender name="TASKLOGFILE" class="ch.qos.logback.classic.sift.SiftingAppender">
        <filter class="org.apache.dolphinscheduler.plugin.task.api.log.TaskLogFilter"/>
        <Discriminator class="org.apache.dolphinscheduler.plugin.task.api.log.TaskLogDiscriminator">
            <key>taskInstanceLogFullPath</key>
            <logBase>${log.base}</logBase>
        </Discriminator>
        <sift>
            <appender name="FILE-${taskInstanceLogFullPath}" class="ch.qos.logback.core.FileAppender">
                <file>${taskInstanceLogFullPath}</file>
                <encoder>
                    <pattern>
                        [%level] %date{yyyy-MM-dd HH:mm:ss.SSS Z} - %message%n
                    </pattern>
                    <charset>UTF-8</charset>
                </encoder>
                <append>true</append>
            </appender>
        </sift>
    </appender>
    <appender name="MASTERLOGFILE" class="ch.qos.logback.core.rolling.RollingFileAppender">
    {{- if .file}}
		<filter class="ch.qos.logback.classic.filter.ThresholdFilter">
			<level>{{.file}}</level>
		</filter>
	{{- end}}
        <file>${log.base}/dolphinscheduler-master.log</file>
        <rollingPolicy class="ch.qos.logback.core.rolling.SizeAndTimeBasedRollingPolicy">
            <fileNamePattern>${log.base}/dolphinscheduler-master.%d{yyyy-MM-dd_HH}.%i.log</fileNamePattern>
            <maxHistory>168</maxHistory>
            <maxFileSize>200MB</maxFileSize>
            <totalSizeCap>50GB</totalSizeCap>
            <cleanHistoryOnStart>true</cleanHistoryOnStart>
        </rollingPolicy>
        <encoder>
            <pattern>
                [WI-%X{workflowInstanceId:-0}][TI-%X{taskInstanceId:-0}] - [%level] %date{yyyy-MM-dd HH:mm:ss.SSS Z} %logger{10}:[%line] - %msg%n
            </pattern>
            <charset>UTF-8</charset>
        </encoder>
    </appender>

	// if custom loggers exists,loop them
{{- range .customLoggers}}
	<logger name="{{.Name}}" level="{{.Level}}"/>
{{- end}}
	
    <root level="INFO">
        <if condition="${DOCKER:-false}">
            <then>
                <appender-ref ref="STDOUT"/>
            </then>
        </if>
        <appender-ref ref="TASKLOGFILE"/>
        <appender-ref ref="MASTERLOGFILE"/>
    </root>
</configuration>`

type Logger struct {
	Name  string
	Level string
}

func TestByTemplate(t *testing.T) {
	customLoggers := []Logger{
		{"test", "INFO"},
		{"test1", "WARN"},
	}
	temp, err := template.New("logback").Parse(origin)
	if err != nil {
		t.Fatal(err)
	}

	var buff bytes.Buffer
	err = temp.Execute(&buff, map[string]interface{}{
		"customLoggers": customLoggers,
		"console":       "info",
		"file":          "warn",
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := `<?xml version="1.0" encoding="UTF-8"?>
<!--
  ~ Licensed to the Apache Software Foundation (ASF) under one or more
  ~ contributor license agreements.  See the NOTICE file distributed with
  ~ this work for additional information regarding copyright ownership.
  ~ The ASF licenses this file to You under the Apache License, Version 2.0
  ~ (the "License"); you may not use this file except in compliance with
  ~ the License.  You may obtain a copy of the License at
  ~
  ~     http://www.apache.org/licenses/LICENSE-2.0
  ~
  ~ Unless required by applicable law or agreed to in writing, software
  ~ distributed under the License is distributed on an "AS IS" BASIS,
  ~ WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  ~ See the License for the specific language governing permissions and
  ~ limitations under the License.
  -->

<configuration scan="true" scanPeriod="120 seconds">
    <property name="log.base" value="logs"/>
    <property scope="context" name="log.base.ctx" value="${log.base}" />

    <appender name="STDOUT" class="ch.qos.logback.core.ConsoleAppender">
		<filter class="ch.qos.logback.classic.filter.ThresholdFilter">
			<level>info</level>
		</filter>
        <encoder>
            <pattern>
                [WI-%X{workflowInstanceId:-0}][TI-%X{taskInstanceId:-0}] - [%level] %date{yyyy-MM-dd HH:mm:ss.SSS Z} %logger{10}:[%line] - %msg%n
            </pattern>
            <charset>UTF-8</charset>
        </encoder>
    </appender>

    <conversionRule conversionWord="message"
                    converterClass="org.apache.dolphinscheduler.common.log.SensitiveDataConverter"/>
    <appender name="TASKLOGFILE" class="ch.qos.logback.classic.sift.SiftingAppender">
        <filter class="org.apache.dolphinscheduler.plugin.task.api.log.TaskLogFilter"/>
        <Discriminator class="org.apache.dolphinscheduler.plugin.task.api.log.TaskLogDiscriminator">
            <key>taskInstanceLogFullPath</key>
            <logBase>${log.base}</logBase>
        </Discriminator>
        <sift>
            <appender name="FILE-${taskInstanceLogFullPath}" class="ch.qos.logback.core.FileAppender">
                <file>${taskInstanceLogFullPath}</file>
                <encoder>
                    <pattern>
                        [%level] %date{yyyy-MM-dd HH:mm:ss.SSS Z} - %message%n
                    </pattern>
                    <charset>UTF-8</charset>
                </encoder>
                <append>true</append>
            </appender>
        </sift>
    </appender>
    <appender name="MASTERLOGFILE" class="ch.qos.logback.core.rolling.RollingFileAppender">
		<filter class="ch.qos.logback.classic.filter.ThresholdFilter">
			<level>warn</level>
		</filter>
        <file>${log.base}/dolphinscheduler-master.log</file>
        <rollingPolicy class="ch.qos.logback.core.rolling.SizeAndTimeBasedRollingPolicy">
            <fileNamePattern>${log.base}/dolphinscheduler-master.%d{yyyy-MM-dd_HH}.%i.log</fileNamePattern>
            <maxHistory>168</maxHistory>
            <maxFileSize>200MB</maxFileSize>
            <totalSizeCap>50GB</totalSizeCap>
            <cleanHistoryOnStart>true</cleanHistoryOnStart>
        </rollingPolicy>
        <encoder>
            <pattern>
                [WI-%X{workflowInstanceId:-0}][TI-%X{taskInstanceId:-0}] - [%level] %date{yyyy-MM-dd HH:mm:ss.SSS Z} %logger{10}:[%line] - %msg%n
            </pattern>
            <charset>UTF-8</charset>
        </encoder>
    </appender>

	// if custom loggers exists,loop them
	<logger name="test" level="INFO"/>
	<logger name="test1" level="WARN"/>
	
    <root level="INFO">
        <if condition="${DOCKER:-false}">
            <then>
                <appender-ref ref="STDOUT"/>
            </then>
        </if>
        <appender-ref ref="TASKLOGFILE"/>
        <appender-ref ref="MASTERLOGFILE"/>
    </root>
</configuration>`

	// assert eq
	assert.Equal(t, expected, buff.String())
}
