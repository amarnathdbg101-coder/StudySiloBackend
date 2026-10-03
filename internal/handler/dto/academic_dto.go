package dto

// Course DTOs
type CreateCourseRequest struct {
	Title       string `json:"title" validate:"required,min=2,max=200"`
	Code        string `json:"code" validate:"max=50"`
	Semester    string `json:"semester" validate:"max=100"`
	Description string `json:"description"`
	ColorHex    string `json:"color_hex" validate:"omitempty,hexcolor"`
	IconName    string `json:"icon_name" validate:"omitempty,max=50"`
}

type UpdateCourseRequest struct {
	Title       *string `json:"title" validate:"omitempty,min=2,max=200"`
	Code        *string `json:"code" validate:"omitempty,max=50"`
	Semester    *string `json:"semester" validate:"omitempty,max=100"`
	Description *string `json:"description"`
	ColorHex    *string `json:"color_hex" validate:"omitempty,hexcolor"`
	IconName    *string `json:"icon_name" validate:"omitempty,max=50"`
	IsArchived  *bool   `json:"is_archived"`
}

// Folder DTOs
type CreateFolderRequest struct {
	CourseID    string  `json:"course_id" validate:"required,uuid"`
	ParentID    *string `json:"parent_id" validate:"omitempty,uuid"`
	Name        string  `json:"name" validate:"required,min=1,max=200"`
	FolderType  string  `json:"folder_type" validate:"omitempty,oneof=module chapter unit custom"`
	Description string  `json:"description"`
	ColorHex    string  `json:"color_hex" validate:"omitempty,hexcolor"`
	IconName    string  `json:"icon_name" validate:"omitempty,max=50"`
	Position    int     `json:"position" validate:"gte=0"`
}

type UpdateFolderRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=1,max=200"`
	ParentID    *string `json:"parent_id" validate:"omitempty,uuid"`
	FolderType  *string `json:"folder_type" validate:"omitempty,oneof=module chapter unit custom"`
	Description *string `json:"description"`
	ColorHex    *string `json:"color_hex" validate:"omitempty,hexcolor"`
	IconName    *string `json:"icon_name" validate:"omitempty,max=50"`
	Position    *int    `json:"position" validate:"omitempty,gte=0"`
}

// Note DTOs
type CreateNoteRequest struct {
	CourseID    *string  `json:"course_id" validate:"omitempty,uuid"`
	FolderID    *string  `json:"folder_id" validate:"omitempty,uuid"`
	ModuleName  string   `json:"module_name" validate:"max=150"`
	Title       string   `json:"title" validate:"required,min=1,max=250"`
	Description string   `json:"description"`
	Content     string   `json:"content"`
	FileURL     string   `json:"file_url"`
	FileType    string   `json:"file_type"`
	FileSize    int64    `json:"file_size" validate:"gte=0"`
	Tags        []string `json:"tags"`
	IsPinned    bool     `json:"is_pinned"`
}

type UpdateNoteRequest struct {
	CourseID    *string   `json:"course_id" validate:"omitempty,uuid"`
	FolderID    *string   `json:"folder_id" validate:"omitempty,uuid"`
	ModuleName  *string   `json:"module_name" validate:"omitempty,max=150"`
	Title       *string   `json:"title" validate:"omitempty,min=1,max=250"`
	Description *string   `json:"description"`
	Content     *string   `json:"content"`
	FileURL     *string   `json:"file_url"`
	FileType    *string   `json:"file_type"`
	FileSize    *int64    `json:"file_size" validate:"omitempty,gte=0"`
	Tags        *[]string `json:"tags"`
	IsPinned    *bool     `json:"is_pinned"`
	IsArchived  *bool     `json:"is_archived"`
}
