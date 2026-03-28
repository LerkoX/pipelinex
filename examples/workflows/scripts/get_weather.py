#!/usr/bin/env python3
import requests
import json
import sys
import os
from datetime import datetime

# OpenMeteo 完全免费，无需 API Key
# 深圳坐标: latitude=22.5431, longitude=114.0579
CITY_COORDS = {
    "深圳": {"lat": 22.5431, "lon": 114.0579, "name": "深圳"},
    "北京": {"lat": 39.9042, "lon": 116.4074, "name": "北京"},
    "上海": {"lat": 31.2304, "lon": 121.4737, "name": "上海"},
    "广州": {"lat": 23.1291, "lon": 113.2644, "name": "广州"},
    "杭州": {"lat": 30.2741, "lon": 120.1551, "name": "杭州"},
    "成都": {"lat": 30.5728, "lon": 104.0668, "name": "成都"},
    "shenzhen": {"lat": 22.5431, "lon": 114.0579, "name": "深圳"},
    "beijing": {"lat": 39.9042, "lon": 116.4074, "name": "北京"},
    "shanghai": {"lat": 31.2304, "lon": 121.4737, "name": "上海"},
}

# WMO 天气代码映射
WEATHER_CODES = {
    0: "晴朗", 1: "多云", 2: "多云", 3: "阴天",
    45: "雾", 48: "雾凇",
    51: "小雨", 53: "小雨", 55: "小雨",
    61: "中雨", 63: "中雨", 65: "中雨",
    71: "小雪", 73: "小雪", 75: "小雪", 77: "雪粒",
    80: "阵雨", 81: "阵雨", 82: "雷阵雨",
    85: "雪阵雨", 86: "雪阵雨",
    95: "雷暴", 96: "雷暴", 99: "雷暴"
}

def get_weather_desc(code):
    """根据 WMO 天气代码获取天气描述"""
    return WEATHER_CODES.get(code, "未知")

def get_rain_chance(code):
    """根据天气代码估算降水概率"""
    if code in [51, 53, 55, 61, 63, 65, 80, 81, 82, 85, 86, 95, 96, 99]:
        return str(min(code % 10 * 10 + 50, 90))
    return "0"

# 从环境变量获取参数
CITY = os.getenv("WEATHER_CITY", "深圳")

print(f"开始获取 {CITY} 未来7天天气预报...")
print(f"获取时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
print()

# 获取城市坐标
city_info = CITY_COORDS.get(CITY.lower(), {"lat": 22.5431, "lon": 114.0579, "name": CITY})
lat, lon, city_name = city_info["lat"], city_info["lon"], city_info["name"]

try:
    # OpenMeteo API - 免费，无需 API Key
    url = (
        f"https://api.open-meteo.com/v1/forecast"
        f"?latitude={lat}&longitude={lon}"
        f"&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m,pressure_msl,apparent_temperature"
        f"&daily=weather_code,temperature_2m_max,temperature_2m_min,apparent_temperature_max,apparent_temperature_min"
        f"&timezone=Asia/Shanghai"
        f"&forecast_days=7"
    )

    response = requests.get(url, timeout=10)
    response.raise_for_status()
    weather_info = response.json()

    # 获取当前天气
    current = weather_info.get("current", {})
    daily = weather_info.get("daily", {})

    print("=== 当前天气 ===")
    print(f"城市: {city_name}")
    print(f"当前温度: {current.get('temperature_2m', '-')}°C")
    print(f"体感: {current.get('apparent_temperature', '-')}°C")
    print(f"天气: {get_weather_desc(current.get('weather_code', 0))}")
    print(f"湿度: {current.get('relative_humidity_2m', '-')}%")
    print(f"风速: {current.get('wind_speed_10m', '-')} km/h")
    print(f"气压: {current.get('pressure_msl', '-')} hPa")
    print()

    # 获取未来7天预报
    dates = daily.get("time", [])
    weather_codes = daily.get("weather_code", [])
    max_temps = daily.get("temperature_2m_max", [])
    min_temps = daily.get("temperature_2m_min", [])
    max_feels = daily.get("apparent_temperature_max", [])
    min_feels = daily.get("apparent_temperature_min", [])

    forecasts = []

    print("=== 未来7日预报 ===")
    for i, date in enumerate(dates):
        try:
            dt = datetime.strptime(date, "%Y-%m-%d")
            weekday = dt.strftime("%A")
        except:
            weekday = ""

        code = weather_codes[i] if i < len(weather_codes) else 0
        max_t = max_temps[i] if i < len(max_temps) else "-"
        min_t = min_temps[i] if i < len(min_temps) else "-"
        feels_max = max_feels[i] if i < len(max_feels) else "-"
        feels_min = min_feels[i] if i < len(min_feels) else "-"

        forecast = {
            "date": date,
            "weekday": weekday,
            "maxTemp": str(max_t) if max_t != "-" else "-",
            "minTemp": str(min_t) if min_t != "-" else "-",
            "avgTemp": str(round((max_t + min_t) / 2, 1)) if isinstance(max_t, (int, float)) and isinstance(min_t, (int, float)) else "-",
            "weather": get_weather_desc(code),
            "chanceofrain": get_rain_chance(code),
            "sunrise": "-",  # OpenMeteo 需要额外参数才能获取日出日落
            "sunset": "-"
        }
        forecasts.append(forecast)

        print(f"  {date} ({weekday}): {min_t}~{max_t}°C, {forecast['weather']}, 降水概率 {forecast['chanceofrain']}%")

    print()
    print('```pipelinex-json')

    # 输出 JSON 数据供后续节点使用
    output = {
        "city": city_name,
        "temp": str(current.get('temperature_2m', '-')),
        "feelsLike": str(current.get('apparent_temperature', '-')),
        "weather": get_weather_desc(current.get('weather_code', 0)),
        "humidity": str(current.get('relative_humidity_2m', '-')),
        "windspeed": str(current.get('wind_speed_10m', '-')),
        "winddir": "-",
        "pressure": str(current.get('pressure_msl', '-')),
        "updateTime": datetime.now().strftime("%H:%M"),
        "forecasts": forecasts
    }

    print(json.dumps(output, ensure_ascii=False))
    print('```')

except Exception as e:
    print(f"获取天气信息失败: {e}")
    import traceback
    traceback.print_exc()
    sys.exit(1)
