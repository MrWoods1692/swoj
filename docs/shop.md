# 商城模块

## 数据表

```sql
CREATE TABLE shop_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT,
  icon TEXT,
  price INTEGER NOT NULL,
  category TEXT,
  active INTEGER DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE shop_orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  item_id INTEGER NOT NULL,
  price INTEGER NOT NULL,
  status INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/shop` | pub | 商品列表（仅 active=1） |
| POST | `/api/shop/redeem` | pri | 下单兑换 |
| GET | `/api/shop/orders` | pri | 我的订单 |
| GET | `/api/admin/shop` | admin | 全部商品（含下架） |
| POST | `/api/admin/shop` | admin | 创建商品 |
| PUT | `/api/admin/shop/{id}` | admin | 更新商品 |
| DELETE | `/api/admin/shop/{id}` | admin | 删除商品 |

## 请求示例

### 下单

```json
POST /api/shop/redeem
{ "item_id": 3 }
```

响应：`{ "order_id": 12, "remaining": 38 }`

### 创建商品

```json
POST /api/admin/shop
{
  "name": "限时头像框",
  "description": "拥有 7 天限时头像框",
  "icon": "frame_star.svg",
  "price": 50,
  "category": "装扮",
  "active": 1
}
```

## 业务规则

- 下单在同一事务：校验余额 → 扣积分 → 写 `shop_orders` → 写 `point_records`（reason=`shop_redeem`）
- 余额不足返回 400「积分不足」
- 商品 `active=0` 不可购买，但已购用户仍可见订单
- 商品删除为软删除（`active=0`），已购订单保留

## 权限

- 公开：读商品列表
- 登录用户：下单、读自己的订单
- 管理员：管理商品

## 前端

`web/src/views/Shop.vue` — 网格展示 + 兑换按钮；`Profile.vue` 显示我的订单。

## 验证

暂无独立 `verify_shop.py`；`verify_logs.py` 覆盖 `shop_redeem` 语义日志。
