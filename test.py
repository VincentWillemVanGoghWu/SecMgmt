import requests
import base64
import mimetypes


def image_to_base64(image_file_path):
    with open(image_file_path, "rb") as f:
        return base64.b64encode(f.read()).decode("utf-8")


API_URL = "http://192.168.1.116:8000/v1/chat/completions"
LOCAL_IMG_PATH = r"D:\test_img.jpg"

mime_type, _ = mimetypes.guess_type(LOCAL_IMG_PATH)
if mime_type is None:
    mime_type = "image/jpeg"

img_b64 = image_to_base64(LOCAL_IMG_PATH)

payload = {
    "model": "Qwen3-VL-8B-Instruct",
    "messages": [
        {
            "role": "user",
            "content": [
                {
                    "type": "image_url",
                    "image_url": {
                        "url": f"data:{mime_type};base64,{img_b64}"
                    },
                },
                {
                    "type": "text",
                    "text": "简要描述图片，提取图中所有文字",
                },
            ],
        }
    ],
    "max_tokens": 400,
    "temperature": 0.4,
}

try:
    resp = requests.post(API_URL, json=payload, timeout=120)
    print("=== 服务器原始返回 ===")
    print(resp.text)

    resp.raise_for_status()
    res_data = resp.json()

    if "error" in res_data:
        print(f"\n接口报错：{res_data['error']['message']}")
    elif "choices" in res_data:
        print("\n模型识别结果：")
        print(res_data["choices"][0]["message"]["content"])
    else:
        print("\n返回数据异常，无推理结果")

except FileNotFoundError:
    print(f"找不到本地图片文件：{LOCAL_IMG_PATH}，检查文件路径是否正确")
except requests.exceptions.RequestException as err:
    print(f"HTTP 请求异常：{err}")
except Exception as err:
    print(f"请求异常：{err}")