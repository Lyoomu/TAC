package server

import (
	"fmt"
	"strings"

	pb "github.com/Lyoomu/TAC/proto"
)

type toolServer struct {
	server *Server
	pb.UnimplementedToolServiceServer
}

func (t *toolServer) DownloadTool(req *pb.DownloadToolRequest, stream pb.ToolService_DownloadToolServer) error {
	toolDetail, err := t.server.toolEngine.GetDetail(req.ToolName)
	if err != nil {
		return err
	}

	files, err := t.server.toolEngine.GetToolFiles(req.ToolName)
	if err != nil {
		return err
	}

	fileCount := 0
	totalFiles := len(files)

	for fileName, content := range files {

		if strings.Contains(fileName, "..") {
			continue
		}
		fileCount++
		isBinary := toolDetail.IsBinary || isBinaryFile(fileName)
		isSource := !isBinary

		if isBinary && !req.DownloadBinary {
			continue
		}
		if isSource && !req.DownloadSource {
			continue
		}

		chunkSize := 32 * 1024
		for offset := 0; offset < len(content); offset += chunkSize {
			end := offset + chunkSize
			if end > len(content) {
				end = len(content)
			}

			chunk := &pb.ToolChunk{
				Data:       content[offset:end],
				FileName:   fileName,
				IsSource:   isSource,
				IsLast:     end >= len(content),
				IsLastFile: fileCount >= totalFiles && end >= len(content),
			}

			if isBinary && chunk.IsLast {
				chunk.WarningMessage = fmt.Sprintf(
					"SECURITY WARNING: Tool '%s' includes a precompiled binary component. "+
						"Please verify the source code before execution. "+
						"Binary execution carries inherent security risks.",
					toolDetail.Name,
				)
			}

			if err := stream.Send(chunk); err != nil {
				return fmt.Errorf("send chunk: %w", err)
			}
		}
	}

	return nil
}

func isBinaryFile(fileName string) bool {

	return len(fileName) > 4 && fileName[:4] == "bin/"
}
