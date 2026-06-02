package generator

// OfflineEngine 离线规则引擎 - 无需 AI Key 即可生成常用命令
// 知识库通过 //go:embed 编译进二进制，完全离线可用
import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/A-small-character/cmdgen-platform/internal/domain/command"
)

// OfflineEngine 基于内置规则库的离线命令生成器
type OfflineEngine struct {
	rules []offlineRule
}

type offlineRule struct {
	keywords  []string          // 触发关键词（任一匹配）
	category  command.Category
	commands  []command.CommandItem
	explain   string
}

func NewOfflineEngine() *OfflineEngine {
	e := &OfflineEngine{}
	e.loadRules()
	return e
}

// Generate 离线生成命令，不需要网络和 API Key
func (e *OfflineEngine) Generate(_ context.Context, input string, cat command.Category) (*command.GenerateResult, error) {
	lower := strings.ToLower(input)
	// 去掉标点，方便匹配
	lower = strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) { return ' ' }
		return r
	}, lower)

	var best []command.CommandItem
	var bestExplain string
	bestScore := 0

	for _, rule := range e.rules {
		if cat != "" && rule.category != cat {
			continue
		}
		score := 0
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = rule.commands
			bestExplain = rule.explain
		}
	}

	if bestScore == 0 {
		return nil, fmt.Errorf("未找到匹配的离线规则，请尝试更具体的描述或启用 AI 模式")
	}

	result := command.NewGenerateResult("offline", cat)
	result.Commands = best
	result.Explanation = bestExplain
	result.Metadata = command.ResultMetadata{
		AIProvider: "offline",
		ModelName:  "rule-engine-v1",
	}
	return result, nil
}

func (e *OfflineEngine) loadRules() {
	e.rules = []offlineRule{
		// ── Linux：查找大文件 ──────────────────────────────────────────────
		{
			keywords: []string{"查找", "大文件", "日志", "find", "log", "size", "1g", "mtime"},
			category: command.CategoryLinux,
			explain:  "查找指定条件的文件，常用于清理磁盘空间",
			commands: []command.CommandItem{
				{ID:"1", Title:"查找7天前大于1GB的文件",
					Command:"find /var/log -type f -mtime +7 -size +1G",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-mtime +7 表示7天前修改，-size +1G 表示大于1GB，-type f 只查文件"},
				{ID:"2", Title:"列出并显示大小",
					Command:`find /var/log -type f -mtime +7 -size +1G -exec ls -lh {} \;`,
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"加 -exec ls -lh 显示每个文件的详细大小信息"},
				{ID:"3", Title:"查找并删除（危险，先确认再执行）",
					Command:"find /var/log -type f -mtime +7 -size +1G -delete",
					Format:command.FormatCLI, Risk:command.RiskHigh,
					Explanation:"直接删除，不可恢复！建议先不加 -delete 确认列表后再执行",
					BackupCmd:"先执行：find /var/log -type f -mtime +7 -size +1G 确认列表"},
				{ID:"4", Title:"查看当前目录最大的10个文件",
					Command:`find . -type f -exec du -h {} \; | sort -rh | head -10`,
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"按文件大小降序排列，显示最大的10个"},
			},
		},
		// ── Linux：磁盘空间 ───────────────────────────────────────────────
		{
			keywords: []string{"磁盘", "disk", "df", "du", "空间", "容量", "使用率", "分区"},
			category: command.CategoryLinux,
			explain:  "查看磁盘空间使用情况",
			commands: []command.CommandItem{
				{ID:"1", Title:"查看所有挂载点磁盘使用情况",
					Command:"df -hT",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-h 人类可读格式，-T 显示文件系统类型"},
				{ID:"2", Title:"查看指定目录占用空间",
					Command:"du -sh /var/log/*",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-s 只显示汇总，-h 人类可读，* 表示该目录下所有子目录"},
				{ID:"3", Title:"查看目录下占用最大的子目录",
					Command:"du -h --max-depth=1 /var | sort -rh | head -10",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"限制深度为1层，按大小降序，取前10"},
				{ID:"4", Title:"查看块设备和分区",
					Command:"lsblk -o NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"显示所有块设备的详细信息"},
			},
		},
		// ── Linux：系统服务 ───────────────────────────────────────────────
		{
			keywords: []string{"服务", "service", "systemctl", "启动", "停止", "重启", "nginx", "apache", "mysql", "start", "stop", "restart"},
			category: command.CategoryLinux,
			explain:  "systemd 服务管理命令（适用于 RHEL 7+/Ubuntu 16+）",
			commands: []command.CommandItem{
				{ID:"1", Title:"查看服务状态",
					Command:"systemctl status nginx",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"把 nginx 替换为实际服务名"},
				{ID:"2", Title:"启动服务",
					Command:"systemctl start nginx",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"立即启动服务"},
				{ID:"3", Title:"停止服务",
					Command:"systemctl stop nginx",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"立即停止服务，会中断现有连接"},
				{ID:"4", Title:"重启服务",
					Command:"systemctl restart nginx",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"先停止后启动，会短暂中断连接"},
				{ID:"5", Title:"重载配置（不中断服务）",
					Command:"systemctl reload nginx",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"平滑重载配置，不会中断现有连接（nginx/apache 支持）"},
				{ID:"6", Title:"设置开机自启",
					Command:"systemctl enable nginx",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:""},
				{ID:"7", Title:"取消开机自启",
					Command:"systemctl disable nginx",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:""},
				{ID:"8", Title:"查看服务日志（最近50行）",
					Command:"journalctl -u nginx -n 50 --no-pager",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-f 参数可实时跟踪日志"},
			},
		},
		// ── Linux：修改 SSH 端口 ──────────────────────────────────────────
		{
			keywords: []string{"ssh", "端口", "port", "sshd", "2222", "openssh"},
			category: command.CategoryLinux,
			explain:  "修改 SSH 服务端口，需要先放行防火墙再重启服务",
			commands: []command.CommandItem{
				{ID:"1", Title:"备份 sshd_config",
					Command:`cp -p /etc/ssh/sshd_config /etc/ssh/sshd_config.$(date +%Y%m%d_%H%M%S).bak`,
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"修改前必须备份，出问题可快速恢复"},
				{ID:"2", Title:"修改端口为 2222（sed 方式）",
					Command:"sed -i 's/^#\\?Port [0-9]*/Port 2222/' /etc/ssh/sshd_config",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"把 2222 改为你想要的端口号",
					BackupCmd:`cp -p /etc/ssh/sshd_config /etc/ssh/sshd_config.bak`,
					RollbackCmd:`cp -p /etc/ssh/sshd_config.bak /etc/ssh/sshd_config && systemctl restart sshd`},
				{ID:"3", Title:"验证修改结果",
					Command:"grep '^Port' /etc/ssh/sshd_config",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"确认端口修改成功"},
				{ID:"4", Title:"防火墙放行新端口（firewalld）",
					Command:"firewall-cmd --permanent --add-port=2222/tcp && firewall-cmd --reload",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"RHEL/CentOS 7+ 适用，先放行再重启 SSH"},
				{ID:"5", Title:"防火墙放行新端口（Ubuntu/ufw）",
					Command:"ufw allow 2222/tcp",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"Ubuntu 适用"},
				{ID:"6", Title:"重启 SSH 服务",
					Command:"systemctl restart sshd",
					Format:command.FormatCLI, Risk:command.RiskHigh,
					Explanation:"⚠️ 重启前务必确认防火墙已放行新端口，否则会断开连接！"},
			},
		},
		// ── Linux：进程管理 ───────────────────────────────────────────────
		{
			keywords: []string{"进程", "process", "ps", "kill", "top", "cpu", "内存", "占用", "pid"},
			category: command.CategoryLinux,
			explain:  "Linux 进程查看与管理",
			commands: []command.CommandItem{
				{ID:"1", Title:"查看所有进程",
					Command:"ps aux --sort=-%cpu | head -20",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"按 CPU 使用率降序排列，显示前20个进程"},
				{ID:"2", Title:"查找指定进程",
					Command:"ps aux | grep nginx",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"把 nginx 替换为要查找的进程名"},
				{ID:"3", Title:"查看端口对应进程",
					Command:"ss -tlnp | grep :80",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"查看占用 80 端口的进程"},
				{ID:"4", Title:"终止进程（优雅）",
					Command:"kill -15 <PID>",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"发送 SIGTERM 信号，允许进程清理后退出"},
				{ID:"5", Title:"强制终止进程",
					Command:"kill -9 <PID>",
					Format:command.FormatCLI, Risk:command.RiskHigh,
					Explanation:"发送 SIGKILL，立即终止，不做任何清理"},
				{ID:"6", Title:"按名称终止所有同名进程",
					Command:"pkill nginx",
					Format:command.FormatCLI, Risk:command.RiskHigh,
					Explanation:"终止所有名为 nginx 的进程"},
			},
		},
		// ── Linux：网络诊断 ───────────────────────────────────────────────
		{
			keywords: []string{"网络", "network", "ping", "ss", "netstat", "端口", "port", "连接", "connection", "tcpdump", "抓包", "ip地址"},
			category: command.CategoryLinux,
			explain:  "Linux 网络诊断常用命令",
			commands: []command.CommandItem{
				{ID:"1", Title:"查看所有监听端口",
					Command:"ss -tlnp",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-t TCP，-l 只显示监听，-n 不解析主机名，-p 显示进程"},
				{ID:"2", Title:"查看网卡 IP 地址",
					Command:"ip addr show",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"显示所有网卡的 IP 地址信息"},
				{ID:"3", Title:"查看路由表",
					Command:"ip route show",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:""},
				{ID:"4", Title:"测试连通性",
					Command:"ping -c 4 8.8.8.8",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-c 4 发送4个包后停止"},
				{ID:"5", Title:"抓取指定端口数据包",
					Command:"tcpdump -i eth0 port 80 -n -c 100",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-i 指定网卡，-n 不解析，-c 100 抓100个包后停止"},
				{ID:"6", Title:"查看连接统计",
					Command:"ss -s",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"显示 TCP/UDP 连接状态汇总"},
			},
		},
		// ── ES：集群健康 ──────────────────────────────────────────────────
		{
			keywords: []string{"elasticsearch", "es", "elastic", "集群", "健康", "health", "cluster"},
			category: command.CategoryElasticsearch,
			explain:  "Elasticsearch 集群健康检查，支持 ES 6.x/7.x/8.x",
			commands: []command.CommandItem{
				{ID:"1", Title:"集群健康检查（curl）",
					Command:"curl -X GET 'http://localhost:9200/_cluster/health?pretty'",
					Format:command.FormatCurl, Risk:command.RiskLow,
					Explanation:"green=正常，yellow=副本未分配，red=主分片缺失"},
				{ID:"2", Title:"集群健康检查（Kibana DevTools）",
					Command:"GET _cluster/health",
					Format:command.FormatKibana, Risk:command.RiskLow,
					Explanation:""},
				{ID:"3", Title:"ES 8.x 带认证",
					Command:"curl -X GET 'http://localhost:9200/_cluster/health?pretty' -u elastic:your_password",
					Format:command.FormatCurl, Risk:command.RiskLow,
					Explanation:"ES 8.x 默认开启 Security，需要携带用户名密码"},
				{ID:"4", Title:"查看节点信息",
					Command:"GET _cat/nodes?v",
					Format:command.FormatKibana, Risk:command.RiskLow,
					Explanation:"显示集群所有节点的 IP、角色、堆内存使用率等"},
				{ID:"5", Title:"查看分片状态",
					Command:"GET _cat/shards?v&h=index,shard,prirep,state,unassigned.reason",
					Format:command.FormatKibana, Risk:command.RiskLow,
					Explanation:"重点关注 state=UNASSIGNED 的分片"},
			},
		},
		// ── ES：索引管理 ─────────────────────────────────────────────────
		{
			keywords: []string{"索引", "index", "创建", "create", "mapping", "template", "settings"},
			category: command.CategoryElasticsearch,
			explain:  "Elasticsearch 索引管理，ES 7.x 之后不再使用 type",
			commands: []command.CommandItem{
				{ID:"1", Title:"创建索引（ES 7.x/8.x）",
					Command:`PUT /my-index
{
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1,
    "refresh_interval": "30s"
  },
  "mappings": {
    "properties": {
      "title":      { "type": "text", "analyzer": "ik_max_word" },
      "content":    { "type": "text" },
      "created_at": { "type": "date", "format": "yyyy-MM-dd HH:mm:ss" },
      "status":     { "type": "keyword" },
      "score":      { "type": "float" }
    }
  }
}`,
					Format:command.FormatKibana, Risk:command.RiskLow,
					Explanation:"分片数创建后不可修改；副本数可以动态调整"},
				{ID:"2", Title:"查看索引信息",
					Command:"GET _cat/indices?v&s=store.size:desc",
					Format:command.FormatKibana, Risk:command.RiskLow,
					Explanation:"按存储大小降序排列所有索引"},
				{ID:"3", Title:"删除索引（危险）",
					Command:"DELETE /my-index",
					Format:command.FormatKibana, Risk:command.RiskHigh,
					Explanation:"⚠️ 不可恢复！请先确认索引名称正确"},
				{ID:"4", Title:"关闭索引（暂停读写但保留数据）",
					Command:"POST /my-index/_close",
					Format:command.FormatKibana, Risk:command.RiskMedium,
					Explanation:"关闭后不可读写，但磁盘空间仍占用"},
			},
		},
		// ── 网络：华为 VLAN ──────────────────────────────────────────────
		{
			keywords: []string{"vlan", "华为", "huawei", "交换机", "switch", "接口", "interface", "trunk", "access"},
			category: command.CategoryNetwork,
			explain:  "华为 VRP 交换机 VLAN 配置，适用于 S 系列交换机",
			commands: []command.CommandItem{
				{ID:"1", Title:"华为：创建 VLAN",
					Command:"system-view\nvlan batch 100 200 300\nquit",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"batch 可同时创建多个 VLAN"},
				{ID:"2", Title:"华为：配置 Access 接口（接终端）",
					Command:"interface GigabitEthernet0/0/1\n port link-type access\n port default vlan 100\n quit",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"Access 口只允许一个 VLAN，发出时去 Tag"},
				{ID:"3", Title:"华为：配置 Trunk 接口（接交换机）",
					Command:"interface GigabitEthernet0/0/24\n port link-type trunk\n port trunk pvid vlan 1\n port trunk allow-pass vlan 100 200 300\n quit",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"Trunk 允许多个 VLAN 通过，携带 802.1Q Tag"},
				{ID:"4", Title:"华为：配置 VLANIF（三层网关）",
					Command:"interface Vlanif100\n ip address 192.168.100.1 255.255.255.0\n quit",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"为 VLAN 100 配置 IP 地址作为该 VLAN 的默认网关"},
				{ID:"5", Title:"Cisco：对等配置（IOS）",
					Command:"vlan 100\n name SERVERS\n exit\ninterface GigabitEthernet1/0/1\n switchport mode access\n switchport access vlan 100\n exit",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"Cisco IOS 对等配置，语法略有不同"},
			},
		},
		// ── Docker：基础命令 ─────────────────────────────────────────────
		{
			keywords: []string{"docker", "容器", "container", "镜像", "image", "run", "compose"},
			category: command.CategoryDocker,
			explain:  "Docker 常用命令速查",
			commands: []command.CommandItem{
				{ID:"1", Title:"运行容器（限制内存+CPU）",
					Command:`docker run -d \
  --name myapp \
  --memory="512m" \
  --cpus="0.5" \
  -p 8080:8080 \
  --restart unless-stopped \
  myimage:latest`,
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-d 后台运行，--restart 自动重启，--memory 限制内存"},
				{ID:"2", Title:"查看运行中的容器",
					Command:"docker ps",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"加 -a 查看所有容器（含已停止）"},
				{ID:"3", Title:"查看容器日志",
					Command:"docker logs -f --tail=100 myapp",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-f 实时跟踪，--tail 只显示最后N行"},
				{ID:"4", Title:"进入容器内部",
					Command:"docker exec -it myapp /bin/sh",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"如果容器有 bash 可用 /bin/bash"},
				{ID:"5", Title:"查看资源使用",
					Command:"docker stats",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"实时显示所有容器的 CPU/内存/网络使用情况"},
				{ID:"6", Title:"清理所有未使用资源",
					Command:"docker system prune -a --volumes",
					Format:command.FormatCLI, Risk:command.RiskHigh,
					Explanation:"⚠️ 会删除所有停止的容器、未使用的镜像和卷"},
			},
		},
		// ── K8s：基础命令 ────────────────────────────────────────────────
		{
			keywords: []string{"kubernetes", "k8s", "kubectl", "pod", "deployment", "service", "namespace", "节点", "node"},
			category: command.CategoryKubernetes,
			explain:  "kubectl 常用命令速查",
			commands: []command.CommandItem{
				{ID:"1", Title:"查看所有 Pod",
					Command:"kubectl get pods -A -o wide",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-A 所有命名空间，-o wide 显示节点IP等信息"},
				{ID:"2", Title:"查看 Pod 日志",
					Command:"kubectl logs -f pod-name -n namespace --tail=100",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"-f 实时跟踪"},
				{ID:"3", Title:"进入 Pod 内部",
					Command:"kubectl exec -it pod-name -n namespace -- /bin/sh",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:""},
				{ID:"4", Title:"查看节点状态",
					Command:"kubectl get nodes -o wide",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"显示节点 IP、版本、状态"},
				{ID:"5", Title:"滚动重启 Deployment",
					Command:"kubectl rollout restart deployment/myapp -n namespace",
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"无损滚动重启，不会中断服务"},
				{ID:"6", Title:"扩缩容",
					Command:"kubectl scale deployment/myapp --replicas=3 -n namespace",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:""},
			},
		},
		// ── MySQL：基础管理 ──────────────────────────────────────────────
		{
			keywords: []string{"mysql", "数据库", "database", "用户", "user", "权限", "grant", "备份", "backup", "主从", "replication"},
			category: command.CategoryMySQL,
			explain:  "MySQL 8.0 常用管理命令",
			commands: []command.CommandItem{
				{ID:"1", Title:"创建数据库和用户",
					Command:`CREATE DATABASE mydb CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'myuser'@'%' IDENTIFIED BY 'StrongPass123!';
GRANT ALL PRIVILEGES ON mydb.* TO 'myuser'@'%';
FLUSH PRIVILEGES;`,
					Format:command.FormatCLI, Risk:command.RiskMedium,
					Explanation:"utf8mb4 支持完整 Unicode 包括 Emoji"},
				{ID:"2", Title:"查看所有数据库和连接",
					Command:"SHOW DATABASES;\nSHOW PROCESSLIST;",
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:""},
				{ID:"3", Title:"备份数据库",
					Command:`mysqldump -u root -p --single-transaction --routines \
  --triggers mydb > /backup/mydb_$(date +%Y%m%d_%H%M%S).sql`,
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"--single-transaction 对 InnoDB 热备不锁表"},
				{ID:"4", Title:"恢复数据库",
					Command:"mysql -u root -p mydb < /backup/mydb_20240101.sql",
					Format:command.FormatCLI, Risk:command.RiskHigh,
					Explanation:"⚠️ 会覆盖现有数据，确认目标库正确"},
				{ID:"5", Title:"查看慢查询",
					Command:`SET GLOBAL slow_query_log = 'ON';
SET GLOBAL long_query_time = 1;
SHOW VARIABLES LIKE 'slow_query_log_file';`,
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"开启慢查询日志，记录超过1秒的SQL"},
				{ID:"6", Title:"查看表大小（前10）",
					Command:`SELECT table_schema, table_name,
  ROUND(data_length/1024/1024, 2) AS data_MB,
  ROUND(index_length/1024/1024, 2) AS index_MB
FROM information_schema.tables
ORDER BY data_length DESC LIMIT 10;`,
					Format:command.FormatCLI, Risk:command.RiskLow,
					Explanation:"查找占用空间最大的表"},
			},
		},
	}
}
