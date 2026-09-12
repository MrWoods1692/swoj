# 全局配置模块

## 数据表

```sql
CREATE TABLE admin_configs (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/admin/config` | admin | 读取全部配置（键值字典） |
| POST | `/api/admin/config` | admin | 保存单项配置 |

## 请求示例

### 读取

```json
GET /api/admin/config
{
  "code": 0,
  "data": {
    "ai.token": "sk-xxxxx",
    "ai.provider": "yunzhi",
    "ai.system_prompt": "...",
    "judge.bin": "/usr/local/bin/go-judge",
    "max_daily_checkin_points": "2",
    "terms.content": "<html>...</html>"
  }
}
```

### 保存

```json
POST /api/admin/config
{ "key": "max_daily_checkin_points", "value": "3" }
```

## 配置项清单

| key | 默认 | 说明 |
|---|---|---|
| `ai.token` | 空 | yunzhiapi.cn Token（覆盖环境变量） |
| `ai.provider` | `yunzhi` | AI 提供商 |
| `ai.system_prompt` | 内置 | AI 系统提示词 |
| `judge.bin` | 空 | go-judge 可执行路径 |
| `terms.content` | 空 | 服务条款 HTML |
| `max_daily_checkin_points` | `1` | 每日签到积分上限 |
| `online_points_per_hour` | `2` | 每小时在线积分 |

## 优先级

`admin_configs` > 环境变量 > 默认值

- AI Token 优先用 `admin_configs.ai.token`；未设置时回退到 `SWOJ_AI_TOKEN`
- 修改配置即时生效；不需要重启服务

## 业务规则

- 保存时校验 `key` 与 `value` 非空
- value 长度上限 16KB；超出返回 400
- 保存后立即写入 access_log（`api:POST /api/admin/config`）；关键项（如 `ai.token`）额外写一条业务日志（`admin_config_update`）

## 权限

- 管理员

## 前端

`web/src/views/Admin.vue` 内嵌「全局配置」面板；AI Token 面板直接读写 `ai.*` 键。

## 验证

暂无独立 verify；通过 `verify_ai.py` 间接覆盖 `ai.token` 的读写。
