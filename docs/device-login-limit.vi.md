# Tính năng giới hạn đăng nhập theo thiết bị

## Mục đích

Mỗi tài khoản chỉ được phép đăng nhập đồng thời trên **1 thiết bị di động** và **1 thiết bị máy tính (PC/Desktop)**. Nếu muốn đăng nhập thêm thiết bị, người dùng cần liên hệ quản trị viên để được cấp quyền.

**System Admin luôn được đăng nhập không giới hạn** — không bị kiểm tra device limit ở bất kỳ bước nào.

---

## Cách hoạt động

### Phân loại thiết bị

| Loại | Điều kiện xác định |
|------|--------------------|
| **Mobile** | Session có `DeviceId` khác rỗng, hoặc `isMobile = true` |
| **Desktop** | Tất cả session còn lại (trình duyệt web, ứng dụng desktop) |

### Quy trình đăng nhập

1. Người dùng gửi yêu cầu đăng nhập.
2. Hệ thống xác thực thông tin (mật khẩu, MFA, …).
3. Nếu là **System Admin** → bỏ qua kiểm tra giới hạn, đăng nhập ngay.
4. Nếu không phải admin:
   - Xóa session cache của người dùng để lấy số liệu chính xác sau logout.
   - Đếm số session **đang hoạt động** (chưa hết hạn) theo từng loại thiết bị.
   - Nếu vượt giới hạn → trả về lỗi **403 Forbidden** với thông báo rõ ràng.
5. Nếu chưa đạt giới hạn → tạo session mới, đăng nhập thành công.

> **Lưu ý:** Đăng nhập lại trên cùng một thiết bị di động (cùng `DeviceId`) không bị tính thêm — session cũ sẽ bị thu hồi trước khi kiểm tra giới hạn.

---

## Giới hạn mặc định

| Loại thiết bị | Giới hạn mặc định |
|---------------|-------------------|
| Mobile        | 1                 |
| Desktop/PC    | 1                 |

Giới hạn được lưu trong trường `Props` (JSONB) của bảng `Users` với 2 key:
- `max_mobile_devices`
- `max_desktop_devices`

Nếu key chưa tồn tại (tài khoản cũ), hệ thống áp dụng giá trị mặc định `1`.

---

## Thông báo lỗi khi đăng nhập

Khi người dùng vượt giới hạn, màn hình đăng nhập hiển thị cảnh báo đúng nội dung (không bị che bởi thông báo "sai mật khẩu"):

- **Mobile bị đầy:**
  > "Mobile device limit reached (1 device(s) allowed). Please ask your administrator to increase the limit or remove a device."

- **Desktop bị đầy:**
  > "Desktop device limit reached (1 device(s) allowed). Please ask your administrator to increase the limit or remove a device."

---

## Quản lý thiết bị trên CMS

Quản trị viên truy cập: **System Console → User Management → [Chọn người dùng]**

Phần **Device Management** xuất hiện ở cuối trang chi tiết người dùng.

### Tóm tắt trạng thái

- **User thường:** hiển thị số session đang hoạt động so với giới hạn:
  ```
  Mobile: 1/1 active  |  Desktop: 0/1 active
  ```
- **System Admin:** hiển thị badge đặc biệt:
  ```
  ∞ System Admin — no device limit
  ```
  Form chỉnh giới hạn bị ẩn với System Admin vì không áp dụng.

### Bảng session đang hoạt động

| Cột | Mô tả |
|-----|-------|
| TYPE | Badge **Mobile** (xanh dương) hoặc **Desktop** (xanh lá). Session hiện tại có thêm chấm tròn vàng và nền row vàng nhạt |
| PLATFORM | Tên OS (đậm) và chi tiết OS · Browser/App phía dưới (xám nhỏ) |
| LAST ACTIVITY | Thời điểm hoạt động cuối |
| (Hành động) | Nút **Remove** để thu hồi session. Bị disabled với session hiện tại |

Bảng có border bo góc, header nền xám nhạt, hover effect trên từng dòng.

#### Chấm tròn "Current"
Session đang được dùng bởi admin đang xem trang sẽ được đánh dấu bằng chấm tròn màu vàng inline sau badge loại thiết bị. Nút Remove bị vô hiệu hóa để tránh tự xóa session đang dùng.

### Chỉnh sửa giới hạn thiết bị

*(Chỉ hiển thị với user không phải System Admin)*

Admin có thể thay đổi giới hạn cho từng tài khoản:
- **Max Mobile Devices**: số thiết bị di động tối đa (1–10)
- **Max Desktop Devices**: số máy tính tối đa (1–10)

Nhấn **Save** (nằm riêng dòng dưới 2 input) để lưu. Thay đổi có hiệu lực với lần đăng nhập tiếp theo.

---

## API Endpoints

Tất cả endpoints yêu cầu xác thực. Xem/chỉnh limit yêu cầu quyền admin phù hợp.

### Lấy danh sách session theo loại thiết bị
```
GET /api/v4/users/{user_id}/device_sessions
```
**Response:**
```json
[
  {
    "id": "abc123",
    "user_id": "xyz789",
    "last_activity_at": 1741824000000,
    "props": {
      "platform": "Linux",
      "os": "Linux",
      "browser": "Chrome/134.0"
    },
    "device_type": "desktop",
    "is_current": true
  }
]
```

> `is_current: true` khi session này chính là session của người đang gọi API (admin đang xem trang).

### Lấy giới hạn thiết bị hiện tại
```
GET /api/v4/users/{user_id}/device_limits
```
**Response (user thường):**
```json
{
  "max_mobile_devices": 1,
  "max_desktop_devices": 2
}
```
**Response (System Admin):**
```json
{
  "max_mobile_devices": 1,
  "max_desktop_devices": 1,
  "bypass_limit": true
}
```

> `bypass_limit: true` khi user là System Admin — không bị giới hạn dù giá trị `max_*` là bao nhiêu.

### Cập nhật giới hạn thiết bị
```
PUT /api/v4/users/{user_id}/device_limits
```
**Body:**
```json
{
  "max_mobile_devices": 2,
  "max_desktop_devices": 1
}
```
**Yêu cầu:** Quyền `Manage System`.

### Thu hồi session (endpoint có sẵn)
```
POST /api/v4/users/{user_id}/sessions/revoke
Body: { "session_id": "abc123" }
```

---

## Các file đã thay đổi

### Backend

| File | Thay đổi |
|------|---------|
| `server/public/model/user.go` | Thêm hằng số `UserPropMaxMobileDevices`, `UserPropMaxDesktopDevices` và 2 method `GetMaxMobileDevices()`, `GetMaxDesktopDevices()` |
| `server/channels/app/login.go` | Thêm `countDeviceSessions()`, kiểm tra giới hạn trong `DoLogin()`. System Admin bypass hoàn toàn. Xóa session cache trước khi đếm để tránh stale data sau logout |
| `server/channels/app/user.go` | Thêm method `UpdateUserDeviceLimits()` |
| `server/channels/api4/device_limits.go` | File mới — 3 handler API: `getUserDeviceSessions` (kèm `is_current`), `getUserDeviceLimits` (kèm `bypass_limit`), `updateUserDeviceLimits` |
| `server/channels/api4/user.go` | Đăng ký 3 route mới; thêm 2 device limit error ID vào `unmaskedErrors` để không bị che bởi thông báo sai mật khẩu |
| `server/i18n/en.json` | Thêm 2 message lỗi cho giới hạn thiết bị |

### Frontend

| File | Thay đổi |
|------|---------|
| `webapp/platform/client/src/client4.ts` | Thêm 3 method: `getUserDeviceSessions`, `getUserDeviceLimits`, `updateUserDeviceLimits` |
| `webapp/.../actions/users.ts` | Thêm 2 Redux action: `getUserDeviceSessions`, `updateUserDeviceLimits` |
| `webapp/.../device_management/device_management.tsx` | Component React: bảng session với type/platform/browser/last activity, chấm tròn current session, nút Remove, form giới hạn. Ẩn form với System Admin, hiển thị badge "∞ no device limit" |
| `webapp/.../device_management/device_management.scss` | Style đầy đủ: bảng có border bo góc, header xám, hover, badge Mobile/Desktop, chấm current, bypass badge |
| `webapp/.../system_user_detail.tsx` | Tích hợp panel Device Management vào trang chi tiết người dùng trong CMS |
| `webapp/.../login.tsx` | Xử lý và hiển thị đúng lỗi giới hạn thiết bị tại màn hình đăng nhập |

---

## Lưu ý kỹ thuật

- **Không cần migration database** — giới hạn lưu vào cột `props` (JSONB) đã có sẵn.
- **System Admin** hoàn toàn bypass ở cả `DoLogin` (server) lẫn CMS UI (ẩn form limit, hiển thị badge vô hạn).
- **Session cache** được xóa (`ClearUserSessionCache`) trước khi đếm để đảm bảo sau khi logout, lần đăng nhập tiếp theo không bị block bởi dữ liệu cũ trong cache.
- **Error masking** — Mattermost mặc định che tất cả lỗi login thành "sai mật khẩu". Device limit errors được thêm vào `unmaskedErrors` để hiển thị đúng thông báo.
- **`is_current`** được xác định ở server trước khi `Sanitize()` xóa token, bằng cách so sánh session ID.
- **Bot và OAuth session** không đi qua `DoLogin()`, không bị tính vào giới hạn.
- **Session hết hạn** không được đếm — `countDeviceSessions()` bỏ qua các session đã `IsExpired()`.
