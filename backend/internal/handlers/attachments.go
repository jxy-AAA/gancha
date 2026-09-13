package handlers

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Attachment 帖子/回答的附件（url 指向本站 /uploads/ 下的文件）。
type Attachment struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

const maxAttachments = 5

// 上传文件命名为 <userID>_<原始文件名>，见 UploadFiles。
var uploadNameRe = regexp.MustCompile(`^[0-9]+_[^/\\?#]+$`)

// marshalAttachments 校验附件并序列化为入库 JSON；返回错误提示（空串表示通过）。
func marshalAttachments(list []Attachment) (string, string) {
	if len(list) == 0 {
		return "", ""
	}
	if len(list) > maxAttachments {
		return "", "附件最多 5 个"
	}
	out := make([]Attachment, 0, len(list))
	for _, a := range list {
		a.Name = strings.TrimSpace(a.Name)
		a.URL = strings.TrimSpace(a.URL)
		if a.Name == "" || len([]rune(a.Name)) > 255 {
			return "", "附件名称不合法"
		}
		if len(a.URL) > 300 || !strings.HasPrefix(a.URL, "/uploads/") ||
			!uploadNameRe.MatchString(strings.TrimPrefix(a.URL, "/uploads/")) {
			return "", "附件地址不合法"
		}
		out = append(out, a)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", "附件格式错误"
	}
	return string(b), ""
}

// attachmentsJSON 把库里的 JSON 文本转为附件数组（解析失败返回空数组）。
func attachmentsJSON(raw string) []Attachment {
	if raw == "" {
		return []Attachment{}
	}
	var out []Attachment
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return []Attachment{}
	}
	return out
}
