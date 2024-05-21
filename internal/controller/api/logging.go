package api

import (
	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewApiLogging(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.ApiRoleGroupSpec) *resource.LoggingRecociler[*dolphinv1alpha1.DolphinschedulerCluster, any] {
	if mergedCfg.Config.Logging == nil {
		return nil
	}
	if loggingSpec := mergedCfg.Config.Logging.Logging; loggingSpec == nil {
		return nil
	}
	loggingContentGenerator := getLoggingDataBuilder(mergedCfg.Config.Logging.Logging)
	logDataBuilder := resource.NewGenericRoleLoggingDataBuilder(getRole(), apiLogbackTpl,
		dolphinv1alpha1.LogbackPropertiesFileName, loggingContentGenerator)
	return resource.NewLoggingReconciler(scheme, instance, client, groupName, labels, mergedCfg, logDataBuilder,
		getRole(), logbackConfigMapName(instance.GetName(), groupName))
}

func getLoggingDataBuilder(loggingSpec *dolphinv1alpha1.LoggingConfigSpec) resource.LoggingContentGenerator {
	return common.TransformApiLogger(loggingSpec)
}

const apiLogbackTpl = `<?xml version="1.0" encoding="UTF-8"?>
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
    {{- if .Console}}
        <filter class="ch.qos.logback.classic.filter.ThresholdFilter">
            <level>{{.Console.Level}}</level>
        </filter>
    {{- end}}
        <encoder>
            <pattern>
                [%level] %date{yyyy-MM-dd HH:mm:ss.SSS Z} %logger{10}:[%line] - %msg%n
            </pattern>
            <charset>UTF-8</charset>
        </encoder>
    </appender>

    <appender name="APILOGFILE" class="ch.qos.logback.core.rolling.RollingFileAppender">
    {{- if .File}}
        <filter class="ch.qos.logback.classic.filter.ThresholdFilter">
            <level>{{.File.Level}}</level>
        </filter>
    {{- end}}
        <file>${log.base}/dolphinscheduler-api.log</file>
        <rollingPolicy class="ch.qos.logback.core.rolling.SizeAndTimeBasedRollingPolicy">
            <fileNamePattern>${log.base}/dolphinscheduler-api.%d{yyyy-MM-dd_HH}.%i.log</fileNamePattern>
            <maxHistory>168</maxHistory>
            <maxFileSize>64MB</maxFileSize>
            <totalSizeCap>50GB</totalSizeCap>
            <cleanHistoryOnStart>true</cleanHistoryOnStart>
        </rollingPolicy>
        <encoder>
            <pattern>
                [%level] %date{yyyy-MM-dd HH:mm:ss.SSS Z} %logger{10}:[%line] - %msg%n
            </pattern>
            <charset>UTF-8</charset>
        </encoder>
    </appender>

    <logger name="org.apache.zookeeper" level="WARN"/>
    <logger name="org.apache.hbase" level="WARN"/>
    <logger name="org.apache.hadoop" level="WARN"/>

{{- range .Loggers}}
    <logger name="{{.Logger}}" level="{{.Level}}"/>
{{- end}}


    <root level="INFO">
        <if condition="${DOCKER:-false}">
            <then>
                <appender-ref ref="STDOUT"/>
            </then>
        </if>
        <appender-ref ref="APILOGFILE"/>
    </root>
</configuration>
`
