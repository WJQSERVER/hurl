# hurl

![License](https://img.shields.io/github/license/WJQSERVER/hurl?color=lightgrey)

> 此项目用于可行性测试

`hurl` 是一个现代化的命令行 HTTP 客户端。它由 Go 语言编写，支持各种 HTTP 方法、认证、代理、自定义 DNS、文件上传下载、JSON 美化输出以及响应体大小限制等高级特性。

## ✨ 特性

*   **直观的命令行接口**: 支持子命令模式 (`hurl <command>`) 和类 `curl` 的**无命令模式** (`hurl <url>`)。
*   **全功能 HTTP 方法**: 支持 `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `HEAD` 等，可通过**子命令名**或在**无命令模式下通过 `-X` Flag** 指定。
*   **灵活的请求体**:
    *   发送原始字符串 (`-d`)。
    *   自动编码 URL-encoded 表单 (`-f`)。
    *   自动编码 JSON 请求 (`-j`)，并自动推断字段类型（数字、布尔、字符串）。
    *   支持 `multipart/form-data` 文件上传。
*   **高级网络配置**:
    *   **代理**: 支持 HTTP/HTTPS 和 SOCKS5/SOCKS5h 代理 (`-x` 统一配置，`--http-proxy`, `--socks5-proxy` 作为遗留 Flag)。
    *   **自定义 DNS**: 使用指定 DNS 服务器进行解析 (`--dns-server`)。
    *   **超时与重试**: 可配置的请求超时 (`--timeout`) 和失败重试次数 (`--retries`)。
*   **丰富的认证方式**: 支持 HTTP Basic Auth (`--user`) 和 Bearer Token (`--bearer`)。
*   **清晰的输出**:
    *   彩色状态码和头部输出。
    *   自动检测并美化 JSON 响应。
    *   可选包含响应头部 (`-i`)。
    *   详细模式 (`-v`) 打印请求细节。
*   **文件操作**:
    *   **下载**: 显示进度条的大文件下载 (`download` 命令)。
    *   **上传**: 方便的文件上传 (`upload` 命令)。
*   **响应体限制**: 可设置最大响应体大小 (`--max-size`)，防止意外下载或打印巨量数据到终端，并为终端输出提供**智能默认限制**。
*   **高度可扩展**: 基于 `httpc` 库构建，易于添加新功能和自定义行为。

## 💡 用法

### 基本格式

`hurl <command> [arguments] [flags]`
或
`hurl <url> [flags]` (无命令模式，默认为 `GET`，带 `-d` `-j` `-f` 则默认为 `POST`)

### 核心命令

#### 发送 GET 请求

```bash
# 最简单的 GET 请求 (无命令模式)
hurl https://example.com

# 使用 get 命令 (推荐)
hurl get https://example.com

# 包含响应头
hurl get -i https://api.example.com/status

# 详细模式 (verbose)
hurl get -v https://example.com/health
```

#### 发送 POST 请求

```bash
# 无命令模式，发送原始文本数据 (自动推断为 POST)
hurl https://api.example.com/data -d "Hello, hurl!"

# 使用 post 命令，发送 JSON 数据 (自动设置 Content-Type: application/json)
hurl post https://api.example.com/users -j name=Alice -j age=30 -j isActive=true

# 使用 post 命令，发送 URL-encoded 表单数据 (自动设置 Content-Type: application/x-www-form-urlencoded)
hurl post https://api.example.com/login -f username=admin -f password=secret
```

#### 文件下载 (`download` 命令)

```bash
# 下载文件到指定位置 (必须使用 -o)
hurl download https://releases.ubuntu.com/jammy/ubuntu-22.04.4-live-server-amd64.iso -o ubuntu.iso

# 下载并限制最大响应体大小 (即便下载也生效)
hurl download https://example.com/large.zip -o large.zip --max-size 10MB
```

#### 文件上传 (`upload` 命令)

```bash
# 上传本地文件 (默认字段名为 'file')
hurl upload https://api.example.com/upload --file /path/to/my/document.txt

# 上传文件并指定表单字段名
hurl upload https://api.example.com/upload --file /path/to/my/image.png --field myImage
```

### 通用 Flag (适用于所有命令和无命令模式)

这些 Flag 在 `hurl` 命令本身 (`hurl <url> [flags]`) 或任何子命令 (`hurl <command> [flags]`) 中都可以使用。

| Flag                  | 描述                                                                            | 示例                                                                  |
| :-------------------- | :------------------------------------------------------------------------------ | :-------------------------------------------------------------------- |
| `--timeout <duration>`| 请求超时时间，如 `10s`, `1m`。默认 `30s`。                                     | `hurl get example.com --timeout 5s`                                   |
| `--retries <count>`   | 请求失败时的最大重试次数。默认 `2`。                                           | `hurl post api.com/flaky -retries 5`                                  |
| `--user-agent <str>`  | 设置自定义 `User-Agent` 头部。默认 `hurl/0.1 Touka HTTP Client/v0`。         | `hurl get example.com --user-agent "MyBrowser/1.0"`                 |
| `-v`                  | 启用详细输出，显示请求细节。                                                    | `hurl get example.com -v`                                             |
| `-i`                  | 包含响应头部在输出中。                                                          | `hurl get example.com -i`                                             |
| `--dns-server <ip:port>` | 使用自定义 DNS 服务器。可多次使用。                                         | `hurl get example.com --dns-server 8.8.8.8:53 --dns-server 1.1.1.1:53`|
| `--user <user:pass>`  | HTTP Basic Auth 认证。                                                          | `hurl get protected.com --user "admin:password"`                      |
| `--bearer <token>`    | Bearer Token 认证。                                                             | `hurl get secured.com --bearer "YOUR_JWT_TOKEN"`                      |
| `-x <proxy_url>`      | 统一代理设置 (HTTP/SOCKS5/SOCKS5h)。优先级最高。                               | `hurl get example.com -x socks5h://localhost:1080`                  |
| `--http-proxy <url>`  | (遗留 Flag) 指定 HTTP/HTTPS 代理。**如果 `-x` 未使用，则此项生效**。             | `hurl get example.com --http-proxy http://localhost:8080`             |
| `--socks5-proxy <url>`| (遗留 Flag) 指定 SOCKS5 代理。**如果 `-x` 和 `--http-proxy` 未使用，则此项生效**。| `hurl get example.com --socks5-proxy socks5://localhost:1080`         |
| `-H <key:value>`      | 添加自定义请求头部。可多次使用。                                                | `hurl get example.com -H "Content-Type: application/json"`            |
| `-f <key=value>`      | 添加 URL-encoded 表单字段。可多次使用。                                       | `hurl post example.com -f name=test`                                  |
| `-j <key=value>`      | 添加 JSON 字段，自动类型推断。可多次使用。                                   | `hurl post example.com -j user=test -j id=123`                        |
| `-d <raw_data>`       | 设置原始请求体。                                                                | `hurl post example.com -d "Hello world"`                              |
| `--max-size <size>`   | 最大响应体大小限制 (如 `10KB`, `5MB`, `1GB`)。终端输出默认 `1MB`。使用 `-1` 为无限制。| `hurl get large.bin --max-size 500KB`                                 |

### 无命令模式专属 Flag

以下 Flag 仅在 `hurl <url> [flags]` 这种无命令模式下生效。

| Flag                  | 描述                                                                            | 示例                                                                  |
| :-------------------- | :------------------------------------------------------------------------------ | :-------------------------------------------------------------------- |
| `-X <method>`         | 指定 HTTP 方法。例如 `hurl example.com -X PUT`。                                | `hurl example.com -X PUT -d "data"`                                   |

### 帮助信息

```bash
# 查看主帮助 (会列出所有命令和通用 Flag)
hurl help
hurl --help
hurl -h

# 查看特定命令的帮助 (会列出该命令特有和通用的 Flag)
hurl <command> --help
hurl get --help
hurl download --help
```


## 📄 许可证

本项目使用 **Mozilla Public License 2.0 (MPL 2.0)**。详见 LICENSE 文件。