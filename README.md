# ip-list

NRO (Number Resource Organization) が公開している最新の統計データ(nro-delegated-stats)から、日本(JP) に割り当てられた IP アドレス範囲を抽出し、ルーターの IP フィルター用リスト等に変換・整形するものです。 \
extracts IP address ranges assigned to Japan (JP) from the latest statistics (nro-delegated-stats) published by the NRO (Number Resource Organization) and converts/formats them into IP filter lists for routers and other network appliances.

## 前提条件 / Requirements

- golang: `v1.25`+

## 使用方法/How to use

### 一覧の取得 / Get list

以下のコマンドで最新の統計データをダウンロードし、解析・整形します。結果は ./output/ に出力されます。 \
Run the following command to download, parse, and format the latest statistics. Results are saved in ./output/.

```bash
go run .
```

- `nro-delegated-stats-YYYYMMDD.txt`
  - JP: NRO から提供される生の統計データ(キャッシュ)
  - EN: Raw statistics provided by NRO (cached).
- `IPv4.txt`,`IPv6.txt`,
  - JP: 日本(JP)に割り当てられた IP アドレスの抽出データ
  - JP: Filtered IP lists assigned to "JP".
- `IPv4-ISP_merged.txt`,`IPv6-ISP_merged.txt`
  - JP: ルーター等のフィルター用に CIDR を統合したデータ
  - JP: Merged CIDR blocks for router filter configurations.

### リストに含まれているかを確認/Check IP by list

指定した IP アドレスまたは CIDR が、生成されたリストに含まれているかを確認できます。 \
You can check if a specific IP address or CIDR exists in the generated list.

```bash
# 特定のIPの確認 / Check IP
go run . 1.1.1.1

# CIDR範囲の確認 / Check CIDR range
go run . 192.168.0.0/24
```
