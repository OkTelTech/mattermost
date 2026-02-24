# Hướng dẫn sử dụng Attendance Bot

Bot chấm công nội bộ tích hợp với Mattermost, hỗ trợ check-in/out, nghỉ giải lao và xin nghỉ phép.

## Mục lục

1. [Dành cho Admin - Thiết lập channels](#dành-cho-admin---thiết-lập-channels)
2. [Dành cho Nhân viên - Sử dụng hàng ngày](#dành-cho-nhân-viên---sử-dụng-hàng-ngày)
3. [Dành cho Quản lý - Duyệt đơn](#dành-cho-quản-lý---duyệt-đơn)

---

## Dành cho Admin - Thiết lập

### Phần A: Setup một lần cho Team

> Các bước này chỉ cần làm **1 lần** khi bắt đầu sử dụng bot cho team.

#### A1. Add Bot vào Team

1. Vào **Team Settings** (click tên team ở góc trái → **Team Settings**)
2. Chọn tab **Members**
3. Click **Add Members**
4. Tìm kiếm `attendance`
5. Click **Add** để thêm bot vào team

> **Lưu ý**: Bot phải được add vào team trước khi có thể add vào các channels trong team đó.

#### A2. Tạo Slash Command `/diemdanh`

Slash command cho phép nhân viên gõ `/diemdanh` để mở menu chấm công.

1. Vào **Main Menu** (☰) → **Integrations**
2. Chọn **Slash Commands** → **Add Slash Command**
3. Điền thông tin:

| Field | Giá trị |
|-------|---------|
| **Title** | Attendance |
| **Description** | Chấm công và xin nghỉ phép |
| **Command Trigger Word** | `diemdanh` |
| **Request URL** | `http://bot-service:3000/api/diemdanh` |
| **Request Method** | POST |
| **Autocomplete** | ✓ Bật |
| **Autocomplete Hint** | (để trống) |
| **Autocomplete Description** | Mở menu chấm công |

4. Click **Save**

---

### Phần B: Setup cho từng nhóm

> Lặp lại các bước này cho **mỗi nhóm/phòng ban** trong team cần sử dụng chấm công.

#### B1. Tạo cặp Channels (Private)

Mỗi nhóm cần **2 channels** (tạo dạng **Private Channel**):

1. Click **+** bên cạnh "Channels" ở sidebar
2. Chọn **Create New Channel**
3. Điền thông tin:
   - **Name**: `attendance-{nhóm}` hoặc `attendance-approval-{nhóm}`
   - **Type**: Chọn **Private** (quan trọng!)
4. Click **Create Channel**

| Channel | Mô tả | Thành viên |
|---------|-------|------------|
| `#attendance-{nhóm}` | Channel chính - thông báo check-in/out, xin nghỉ | Tất cả nhân viên trong nhóm |
| `#attendance-approval-{nhóm}` | Channel duyệt - có nút Approve/Reject | Chỉ quản lý/team lead |

> **Tại sao dùng Private Channel?**
> - Nhân viên không thể tự join channel - phải được mời
> - Channel approval chỉ có quản lý mới thấy được
> - Kiểm soát được ai có quyền duyệt đơn

**Ví dụ trong 1 team có nhiều nhóm:**

```
Team Engineering:
├── 🔒 #attendance-frontend        → Frontend developers
├── 🔒 #attendance-approval-frontend → Frontend Lead
├── 🔒 #attendance-backend         → Backend developers
├── 🔒 #attendance-approval-backend  → Backend Lead
├── 🔒 #attendance-devops          → DevOps engineers
└── 🔒 #attendance-approval-devops   → DevOps Lead

Team Sales:
├── 🔒 #attendance-sales-north     → Sales miền Bắc
├── 🔒 #attendance-approval-sales-north → Manager miền Bắc
├── 🔒 #attendance-sales-south     → Sales miền Nam
└── 🔒 #attendance-approval-sales-south → Manager miền Nam
```

#### B2. Add Bot vào Channels

Sau khi tạo channels, add bot vào **cả 2 channels**:

1. Mở channel (ví dụ: `#attendance-frontend`)
2. Click **biểu tượng ⓘ** (Channel Info) ở góc phải
3. Click **Members** → **Add Members**
4. Tìm kiếm `attendance`
5. Click **Add** để thêm bot vào channel

**Lặp lại cho cả 2 channels:**
- `#attendance-{nhóm}` - Bot cần ở đây để gửi thông báo check-in/out
- `#attendance-approval-{nhóm}` - Bot cần ở đây để gửi đơn với nút Approve/Reject

> **Quan trọng**: Nếu bot không được add vào channel, bot sẽ không thể gửi tin nhắn hoặc tạo bài post trong channel đó!

#### B3. Mời thành viên vào Channels

- **Channel chính** (`#attendance-{nhóm}`): Mời tất cả nhân viên trong nhóm
- **Channel approval** (`#attendance-approval-{nhóm}`): Chỉ mời quản lý/người có quyền duyệt

#### B4. Quản lý người duyệt đơn

Việc quản lý người duyệt đơn = quản lý thành viên channel approval:

- **Thêm người duyệt**: Mời họ vào channel `#attendance-approval-{nhóm}`
- **Xóa quyền duyệt**: Kick họ khỏi channel `#attendance-approval-{nhóm}`

Không cần cấu hình gì thêm!

---

## Dành cho Nhân viên - Sử dụng hàng ngày

### Mở menu chấm công

Vào channel `#attendance-{team}` của bạn và gõ:

```
/attendance
```

Một menu sẽ hiện ra (chỉ bạn thấy):

```
┌─────────────────────────────────────────────────────────┐
│ Attendance                                              │
│ [Check In] [Break] [End Break] [Check Out]              │
│                                                         │
│ Requests                                                │
│ [Leave Request] [Late Arrival] [Early Departure]        │
└─────────────────────────────────────────────────────────┘
```

### Attendance - Chấm công

#### Check In (Bắt đầu làm việc)
1. Gõ `/attendance`
2. Click nút **[Check In]**
3. Hệ thống ghi nhận giờ vào và thông báo cho cả channel:
   ```
   ✓ @username đã check-in lúc 08:30
   ```

#### Break (Nghỉ giải lao)
1. Gõ `/attendance`
2. Click nút **[Break]** để bắt đầu nghỉ
3. Khi quay lại làm việc, gõ `/attendance` và click **[End Break]**

#### Check Out (Kết thúc ngày làm việc)
1. Gõ `/attendance`
2. Click nút **[Check Out]**
3. Hệ thống ghi nhận giờ ra:
   ```
   ✓ @username đã check-out lúc 17:30
   ```

### Requests - Yêu cầu

#### Leave Request (Xin nghỉ phép)
1. Gõ `/attendance`
2. Click nút **[Leave Request]**
3. Điền form:
   ```
   ┌─────────────────────────────────────┐
   │        Đơn xin nghỉ phép            │
   │  Loại:     [Nghỉ phép năm ▼]        │
   │  Từ ngày:  [2026-01-30]             │
   │  Đến ngày: [2026-01-31]             │
   │  Lý do:    [________________]       │
   │                                     │
   │              [Hủy] [Gửi]            │
   └─────────────────────────────────────┘
   ```

4. Click **[Gửi]**

5. Đơn của bạn sẽ hiển thị trong channel (mọi người thấy):
   ```
   ┌─────────────────────────────────────┐
   │ ĐƠN XIN NGHỈ #LR-2026013001         │
   │ Người gửi: @username                │
   │ Loại: Nghỉ phép năm                 │
   │ Ngày: 30/01 → 31/01 (2 ngày)        │
   │ Lý do: Việc gia đình                │
   │ Trạng thái: ĐANG CHỜ DUYỆT          │
   └─────────────────────────────────────┘
   ```

6. Quản lý sẽ nhận được thông báo trong channel `#attendance-approval-{team}` với nút duyệt

#### Late Arrival (Xin đi muộn)
1. Gõ `/attendance`
2. Click nút **[Late Arrival]**
3. Điền form:
   - Ngày đi muộn
   - Giờ dự kiến đến
   - Lý do
4. Click **[Gửi]**

#### Early Departure (Xin về sớm)
1. Gõ `/attendance`
2. Click nút **[Early Departure]**
3. Điền form:
   - Ngày về sớm
   - Giờ dự kiến về
   - Lý do
4. Click **[Gửi]**

### Theo dõi trạng thái đơn

Khi đơn được duyệt/từ chối, bạn sẽ nhận được:
- Thông báo trong channel `#attendance-{team}`
- Tin nhắn riêng (DM) từ bot

**Ví dụ khi được duyệt:**
```
Đơn nghỉ phép #LR-2026013001 của bạn đã được DUYỆT bởi @teamlead
```

**Ví dụ khi bị từ chối:**
```
Đơn nghỉ phép #LR-2026013001 của bạn đã bị TỪ CHỐI bởi @teamlead
Lý do: Trùng lịch dự án quan trọng
```

---

## Dành cho Quản lý - Duyệt đơn

### Nhận đơn cần duyệt

Khi nhân viên gửi đơn (nghỉ phép, đi muộn, về sớm), bạn sẽ thấy trong channel `#attendance-approval-{team}`:

```
┌─────────────────────────────────────┐
│ YÊU CẦU #LR-2026013001              │
│ Người gửi: @nguyenvana              │
│ Loại: Nghỉ phép năm                 │
│ Ngày: 30/01 → 31/01 (2 ngày)        │
│ Lý do: Việc gia đình                │
│ Trạng thái: ĐANG CHỜ DUYỆT          │
│                                     │
│      [Duyệt]  [Từ chối]             │
└─────────────────────────────────────┘
```

### Duyệt đơn

1. Click nút **[Duyệt]**
2. Hệ thống sẽ:
   - Cập nhật trạng thái đơn thành **ĐÃ DUYỆT**
   - Thông báo cho nhân viên qua channel và DM
   - Xóa nút bấm khỏi message

### Từ chối đơn

1. Click nút **[Từ chối]**
2. Điền lý do từ chối (bắt buộc)
3. Hệ thống sẽ:
   - Cập nhật trạng thái đơn thành **TỪ CHỐI**
   - Thông báo cho nhân viên kèm lý do

### Lưu ý khi duyệt

- **Không thể tự duyệt đơn của chính mình** - Hệ thống sẽ báo lỗi
- **Chỉ duyệt được đơn đang chờ** - Đơn đã duyệt/từ chối không thể thay đổi
- **Mọi hành động được ghi log** - Ai duyệt, lúc nào đều được lưu lại

