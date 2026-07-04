Sử dụng Redis để lưu trữ cấu trúc dữ liệu cấu hình người dùng

#### I. Cấu hình

##### 1. Cấu trúc hget của cấu hình toàn cục

milestones:global:config

##### 2. Cấu hình người dùng có thể ghi đè lên cấu hình trong file config, cấu trúc hget

```
milestones:userconfig:{deviceid}
    "llm": {
        "provider": "deepseek",         // tương ứng với key trong mục llm của file cấu hình
    },
    "tts": {
        "provider": "cosyvoice",        // tương ứng với key trong mục tts của file cấu hình
    }
```

#### II. Prompt

##### 1. Get/set prompt hệ thống (system prompt)

> milestones:llm:system:{deviceid}

##### 2. Cấu trúc sorted set lưu lịch sử prompt của phiên chat (chat session)

> milestones:llm:{deviceid}
