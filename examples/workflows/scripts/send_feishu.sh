#!/bin/bash
set -e

echo "准备发送飞书通知..."
echo ""

if [ -z "{{ Param.feishuWebhook }}" ]; then
  echo "警告: 飞书webhook地址未配置"
  echo ""
  echo "请在配置文件中设置 Param.feishuWebhook 参数"
  echo ""
  echo "配置示例:"
  echo "  Param:"
  echo "    city: \"深圳\""
  echo "    feishuWebhook: \"https://open.feishu.cn/open-apis/bot/v2/hook/xxxxxxxx\""
  echo ""
  echo "获取飞书机器人webhook步骤:"
  echo "1. 在在飞书中创建自定义机器人"
  echo "2. 获取webhook地址"
  echo "3. 将地址填入配置文件"
  exit 1
fi

echo "Webhook地址已配置: {{ Param.feishuWebhook }}"
echo ""
echo "发送消息内容:"
echo "城市: {{ Param.city }}"
echo "温度: {{ GetWeather.temp }}°C"
echo "天气: {{ GetWeather.weather }}"
echo "湿度: {{ GetWeather.humidity }}%"
echo "风速: {{ GetWeather.windspeed }} km/h"
echo ""

# 构建飞书消息
message=$(cat << EOF
{
  "msg_type": "post",
  "content": {
    "post": {
      "zh_cn": {
        "title": "深圳天气播报",
        "content": [
          [
            {
              "tag": "text",
              "text": "时间: $(date '+%Y-%m-%d %H:%M:%S')\\n"
            }
          ],
          [
            {
              "tag": "text",
              "text": "城市: {{ Param.city }}\\n"
            }
          ],
          [
            {
              "tag": "text",
              "text": "温度: {{ GetWeather.temp }}°C\\n"
            }
          ],
          [
            {
              "tag": "text",
              "text": "天气: {{ GetWeather.weather }}\\n"
            }
          ],
          [
            {
              "tag": "text",
              "text": "湿度: {{ GetWeather.humidity }}%\\n"
            }
          ],
          [
            {
              "tag": "text",
              "text": "风速: {{ GetWeather.windspeed }} km/h\\n"
            }
          ]
        ]
      }
    }
  }
}
EOF
)

echo "$message" | jq '.' 2>/dev/null || echo "消息JSON格式化失败"
echo ""
echo "=== 发送飞书消息 ==="
# 实际发送（注释掉以避免在测试时发送）
# response=$(curl -s -X POST "{{ Param.feishuWebhook}}" -H "Content-Type: application/json" -d "$message")
# echo "飞书响应: $response"
echo "消息内容已准备好，取消注释以实际发送"
