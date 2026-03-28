#!/usr/bin/env python3
import requests
import json
import sys
import os
from datetime import datetime

# 从环境变量获取配置
APP_ID = os.getenv("FEISHU_APP_ID", "{{ Param.feishuAppId }}")
APP_SECRET = os.getenv("FEISHU_APP_SECRET", "{{ Param.feishuAppSecret }}")
CHAT_ID = os.getenv("FEISHU_CHAT_ID", "{{ Param.feishuChatId }}")
WEATHER_DATA_FILE = os.getenv("WEATHER_DATA_FILE", "/tmp/weather_data.json")

def get_access_token(app_id, app_secret):
    """获取 tenant_access_token"""
    url = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
    headers = {"Content-Type": "application/json"}
    data = {
        "app_id": app_id,
        "app_secret": app_secret
    }
    response = requests.post(url, headers=headers, json=data)
    result = response.json()
    if result.get("code") != 0:
        raise Exception(f"获取 token 失败: {result}")
    return result.get("tenant_access_token")

def send_card_message(token, chat_id, title, text_content):
    """发送卡片消息到群聊（支持 Markdown）"""
    url = "https://open.feishu.cn/open-apis/im/v1/messages"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }
    params = {"receive_id_type": "chat_id"}

    # 构建卡片消息内容
    card_content = {
        "config": {
            "wide_screen_mode": True
        },
        "header": {
            "template": "blue",
            "title": {
                "content": title,
                "tag": "plain_text"
            }
        },
        "elements": [
            {
                "tag": "div",
                "text": {
                    "content": text_content,
                    "tag": "lark_md"
                }
            }
        ]
    }

    data = {
        "receive_id": chat_id,
        "msg_type": "interactive",
        "content": json.dumps(card_content, ensure_ascii=False)
    }

    response = requests.post(url, headers=headers, params=params, json=data)
    return response.json()

def build_weather_content(weather_data):
    """构建天气报告的 Markdown 内容"""
    city = weather_data.get("city", "未知")
    temp = weather_data.get("temp", "-")
    feels_like = weather_data.get("feelsLike", "-")
    weather = weather_data.get("weather", "-")
    humidity = weather_data.get("humidity", "-")
    windspeed = weather_data.get("windspeed", "-")
    update_time = weather_data.get("updateTime", "-")
    forecasts = weather_data.get("forecasts", [])

    content = f"**🌤️ {city} 天气报告**\n\n"
    content += f"**📊 当前天气**\n"
    content += f"- **温度**: {temp}°C (体感 {feels_like}°C)\n"
    content += f"- **天气**: {weather}\n"
    content += f"- **湿度**: {humidity}%\n"
    content += f"- **风速**: {windspeed} km/h\n"
    content += f"- **更新时间**: {update_time}\n\n"

    if forecasts:
        content += f"**📅 未来 {len(forecasts)} 天预报**\n\n"
        for f in forecasts:
            date = f.get("date", "")
            weekday = f.get("weekday", "")
            min_temp = f.get("minTemp", "-")
            max_temp = f.get("maxTemp", "-")
            w = f.get("weather", "-")
            rain_chance = f.get("chanceofrain", "0")
            content += f"**🗓️ {date} ({weekday})**\n"
            content += f"🌡️ 温度: {min_temp} ~ {max_temp}°C  |  ☁️ 天气: {w}  |  🌧️ 降水: {rain_chance}%\n\n"

    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    content += f"🕒 报告生成时间: {now}"

    return content

def main():
    print("准备发送飞书天气报告...")
    print()

    # 验证配置
    if not APP_ID or APP_ID.startswith("{{"):
        print("警告: 飞书 AppID 未配置")
        sys.exit(1)

    if not APP_SECRET or APP_SECRET.startswith("{{"):
        print("警告: 飞书 AppSecret 未配置")
        sys.exit(1)

    if not CHAT_ID or CHAT_ID.startswith("{{"):
        print("警告: 飞书群聊ID 未配置")
        sys.exit(1)

    # 从文件读取天气数据
    try:
        with open(WEATHER_DATA_FILE, 'r', encoding='utf-8') as f:
            content = f.read()
            # 提取 pipelinex-json 代码块中的 JSON
            import re
            match = re.search(r'```pipelinex-json\s*\n(.*?)\n```', content, re.DOTALL)
            if match:
                json_str = match.group(1)
                weather_data = json.loads(json_str)
            else:
                # 如果没有代码块包装，直接解析
                weather_data = json.loads(content)

        print(f"天气数据: 城市={weather_data.get('city', '未知')}, 温度={weather_data.get('temp', '-')}°C")
        forecasts = weather_data.get("forecasts", [])
        print(f"未来预报天数: {len(forecasts)}")
        print()
    except Exception as e:
        print(f"读取天气数据失败: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

    try:
        # 获取 token
        token = get_access_token(APP_ID, APP_SECRET)
        print("Token 获取成功")

        # 构建消息内容
        title = f"{weather_data.get('city', '天气')} 每日播报"
        content = build_weather_content(weather_data)

        print("=== 发送飞书消息 ===")
        print(f"标题: {title}")
        print(f"消息内容长度: {len(content)} 字符")
        print(f"消息内容:\n{content}")

        # 发送消息
        result = send_card_message(token, CHAT_ID, title, content)

        if result.get("code") == 0:
            print("✅ 消息发送成功！")
        else:
            print(f"❌ 消息发送失败: {result}")
            sys.exit(1)

    except Exception as e:
        print(f"发送失败: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == "__main__":
    main()
