#!/bin/bash
set -e

echo "格式化飞书消息..."
echo ""

# 从环境变量获取参数
temp="${WEATHER_TEMP:-}"
weather="${WEATHER_WEATHER:-}"
humidity="${WEATHER_HUMIDITY:-}"
windspeed="${WEATHER_WINDSPEED:-}"

echo '```pipelinex-json'
echo '{
  "msg_type": "interactive",
  "card": {
    "header": {
      "title": {
        "tag": "plain_text",
        "content": "深圳天气播报"
      },
      "template": "blue"
    },
    "elements": [
      {
        "tag": "div",
        "text": {
          "tag": "lark_md",
          "content": "时间: '"$(date '+%Y-%m-%d %H:%M:%S')"'"\\n温度: '"${temp}"'°C\\n天气: '"${weather}"'\\n湿度: '"${humidity}"'%\\n风速: '"${windspeed}"' km/h"
        }
      }
    ]
  }
}'
echo '```'
