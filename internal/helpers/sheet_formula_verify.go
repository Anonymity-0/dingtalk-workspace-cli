package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newSheetFormulaVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "formula-verify",
		Short: "校验表格公式错误",
		Long: `扫描钉钉电子表格中已落表的公式单元格，按计算结果错误类型聚合返回错误数量、位置和样本。

不指定 --sheet-id / --range / --targets 时默认扫描整本表格的全部工作表。`,
		Example: `  dws sheet formula-verify --node NODE_ID
  dws sheet formula-verify --node NODE_ID --sheet-id Sheet1 --range A1:D100
  dws sheet formula-verify --node NODE_ID --exit-on-error`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRequiredFlags(cmd, "node"); err != nil {
				return err
			}
			toolArgs := map[string]any{
				"nodeId": mustGetFlag(cmd, "node"),
			}
			targets, err := formulaVerifyTargetsFromFlags(cmd)
			if err != nil {
				return err
			}
			if len(targets) > 0 {
				toolArgs["targets"] = targets
			}
			if cmd.Flags().Changed("max-locations-per-error") {
				v, _ := cmd.Flags().GetInt("max-locations-per-error")
				if v <= 0 {
					return fmt.Errorf("--max-locations-per-error 必须是正整数")
				}
				toolArgs["maxLocationsPerError"] = v
			}
			if cmd.Flags().Changed("max-cells") {
				v, _ := cmd.Flags().GetInt("max-cells")
				if v <= 0 {
					return fmt.Errorf("--max-cells 必须是正整数")
				}
				toolArgs["maxCells"] = v
			}
			exitOnError, _ := cmd.Flags().GetBool("exit-on-error")
			if err := callMCPToolOnServer("sheet", "formula_verify", toolArgs); err != nil {
				return err
			}
			if exitOnError {
				return fmt.Errorf("formula errors detected (--exit-on-error)")
			}
			return nil
		},
	}
	cmd.Flags().String("node", "", "表格文档 ID 或 URL (必填)")
	cmd.Flags().String("sheet-id", "", "工作表 ID 或名称")
	cmd.Flags().String("range", "", "A1 范围（需与 --sheet-id 配合）")
	cmd.Flags().String("targets", "", `扫描目标 JSON 数组或 @文件路径；每项 {"sheetId":"Sheet1","range":"A1:D100"}`)
	cmd.Flags().Int("max-locations-per-error", 0, "每种错误类型最多返回的位置数")
	cmd.Flags().Int("max-cells", 0, "最多扫描的单元格数")
	cmd.Flags().Bool("exit-on-error", false, "发现公式错误时返回非 0 退出码")
	return cmd
}

func formulaVerifyTargetsFromFlags(cmd *cobra.Command) ([]map[string]any, error) {
	if v, _ := cmd.Flags().GetString("targets"); v != "" {
		return parseFormulaVerifyTargets(v)
	}
	sheetID, _ := cmd.Flags().GetString("sheet-id")
	rangeStr, _ := cmd.Flags().GetString("range")
	if sheetID != "" {
		t := map[string]any{"sheetId": sheetID}
		if rangeStr != "" {
			t["range"] = rangeStr
		}
		return []map[string]any{t}, nil
	}
	return nil, nil
}

func parseFormulaVerifyTargets(raw string) ([]map[string]any, error) {
	data := raw
	if strings.HasPrefix(raw, "@") {
		filePath := strings.TrimPrefix(raw, "@")
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取 --targets 文件失败: %w", err)
		}
		data = string(content)
	} else if raw == "-" {
		content, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return nil, fmt.Errorf("读取 stdin 失败: %w", err)
		}
		data = string(content)
	}
	var targets []map[string]any
	if err := json.Unmarshal([]byte(data), &targets); err != nil {
		return nil, fmt.Errorf("--targets JSON 解析失败: %w", err)
	}
	return targets, nil
}
