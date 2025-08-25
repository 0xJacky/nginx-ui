package stream

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
)

// StreamInfo represents stream information
type Info struct {
	Path       string
	Status     config.Status
	Model      *model.Stream
	FileInfo   os.FileInfo
	RawContent string
	NgxConfig  *nginx.NgxConfig
}

// GetStreamInfo retrieves comprehensive information about a stream
func GetStreamInfo(name string) (*Info, error) {
	// Get the absolute path to the stream configuration file
	path := nginx.GetConfPath("streams-available", name)
	fileInfo, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, ErrStreamNotFound
	}
	if err != nil {
		return nil, err
	}

	// Check if the stream is enabled
	status := config.StatusEnabled
	if _, err := os.Stat(nginx.GetConfPath("streams-enabled", name)); os.IsNotExist(err) {
		status = config.StatusDisabled
	}

	// Retrieve or create stream model from database
	s := query.Stream
	streamModel, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		return nil, err
	}

	// Read raw content
	rawContent, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	info := &Info{
		Path:       path,
		Status:     status,
		Model:      streamModel,
		FileInfo:   fileInfo,
		RawContent: string(rawContent),
	}

	// Parse configuration if not in advanced mode
	if !streamModel.Advanced {
		nginxConfig, err := nginx.ParseNgxConfig(path)
		if err != nil {
			return nil, err
		}
		info.NgxConfig = nginxConfig
	}

	return info, nil
}

// SaveStreamConfig saves stream configuration with database update
func SaveStreamConfig(name, content string, namespaceID uint64, syncNodeIDs []uint64, overwrite bool, postAction string) error {
	// Get stream from database or create if not exists
	path := nginx.GetConfPath("streams-available", name)
	s := query.Stream
	streamModel, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		return err
	}

	// Update Namespace ID if provided
	if namespaceID > 0 {
		streamModel.NamespaceID = namespaceID
	}

	// Update synchronization node IDs if provided
	if syncNodeIDs != nil {
		streamModel.SyncNodeIDs = syncNodeIDs
	}

	// Save the updated stream model to database
	_, err = s.Where(s.ID.Eq(streamModel.ID)).Updates(streamModel)
	if err != nil {
		return err
	}

	// Save the stream configuration file
	return Save(name, content, overwrite, syncNodeIDs, postAction)
}
