package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/codeskyblue/go-sh"
	"github.com/fsnotify/fsnotify"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

var configFile string

type Project struct {
	Name   string // 项目名称
	Repo   string // 仓库地址
	Branch string // 同步分支
	Dir    string // 本地目录
	Notify string // 通知地址

}

type Config struct {
	Project []Project // 项目列表
	Notify  string    // 通知地址 tg:xxx lark:xxx pushdeer:xxx
}

var C *Config

func main() {
	flag.StringVar(&configFile, "c", "deploy.yaml", "config file")
	flag.Parse()

	C := &Config{}

	viper.SetConfigType("yaml")
	viper.SetConfigFile(configFile)
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	err = viper.Unmarshal(&C)
	if err != nil {
		panic(err)
	}

	viper.OnConfigChange(func(in fsnotify.Event) {
		err := viper.Unmarshal(&C)
		if err != nil {
			slog.Error("reload config error", "err", err)
		}
		slog.Info("reload config")
	})

	viper.WatchConfig()

	//for {
	for _, p := range C.Project {
		Deploy(&p)
	}
	time.Sleep(5 * time.Second)
	//}
}

func Deploy(p *Project) {
	dir := p.Dir

	s := sh.NewSession()

	needDeploy := false

	if !CheckFileExists(dir) {
		_, err := s.Command("git", "clone", "--depth", "1", p.Repo, dir).Output()
		if err != nil {
			slog.Error("git clone", "err", err)
			return
		}
		needDeploy = true
	} else {
		s = s.SetDir(dir)
		_, err := s.Command("git", "fetch", "origin", p.Branch).Output()
		if err != nil {
			slog.Error("git pull", "err", err)
			return
		}

		out, err := s.
			Command("git", "rev-list", "--count", "HEAD..origin/"+p.Branch).
			Output()
		if err != nil {
			slog.Error("git rev-list --count HEAD..origin/"+p.Branch, "err", err)
			return
		}
		if cast.ToInt(out) == 0 {
			slog.Info("no new commit")
			return
		}
		_, err = s.Command("git", "fetch", "origin", p.Branch).Output()
		if err != nil {
			slog.Error("git fetch origin main", "err", err)
			return
		}

		_, err = s.Command("git", "pull", "origin", p.Branch).Output()
		if err != nil {
			slog.Error("git pull origin"+p.Branch, "err", err)
			return
		}

		needDeploy = true
	}

	if !needDeploy {
		return
	}

	var port string

	if CheckFileExists(filepath.Join(dir, "Dockerfile")) {
		_, err := s.Command("docker", "build", "-t", p.Name+":latest", ".").Output()
		if err != nil {
			slog.Error("docker build", "err", err)
			return
		}

		// 使用正则表达式匹配EXPOSE指令
		re := regexp.MustCompile(`EXPOSE (\d+)(?:/\w+)?`)
		matches := re.FindAllStringSubmatch(GetFileContent(filepath.Join(dir, "Dockerfile")), -1)

		// 遍历匹配结果，提取端口
		for _, match := range matches {
			slog.Info("exported", "port", match[1])
			port = match[1]
		}
	}

	if CheckFileExists(filepath.Join(dir, "docker-compose.yml")) {
		_, err := s.Command("docker-compose", "up", "-d", "--remove-orphans").Output()
		if err != nil {
			slog.Error("docker-compose up -d --remove-orphans", err)
			return
		}

		out, err := s.Command("docker", "compose", "ps", "-q").Output()
		if err != nil {
			slog.Error("docker compose ps -q", err)
			return
		}
		slog.Info("service id", string(out))
	} else {
		containerId, err := s.Command("docker", "ps", "-q", "-f", "name="+p.Name).Output()
		if err != nil {
			slog.Error("docker ps -q -f name="+p.Name, "err", err)
			return
		}

		if len(containerId) > 0 {
			_, err := s.Command("docker", "stop", string(containerId)).Output()
			if err != nil {
				slog.Error("docker stop", "err", err)
				return
			}
			slog.Info("docker stop", "containerId", string(containerId))
		}

		containerName := p.Name + "-easy-deploy"

		slog.Info(fmt.Sprintf("docker run -d -p %s:%s -name %s %s:latest", port, port, containerName, p.Name))
		_, err = s.Command("docker", "run", "-d", "-p", port+":"+port, "--name", containerName, p.Name+":latest").Output()
		if err != nil {
			slog.Error("docker run", "err", err)
			return
		}
		slog.Info("docker run", "name", containerName)
	}

	_notify := p.Notify
	if _notify == "" {
		_notify = C.Notify
	}

	if _notify != "" {
		notify("部署完成", _notify)
	}
}

func notify(msg string, notify string) {
	hostname, _ := os.Hostname()
	msg = fmt.Sprintf("[%s] %s", hostname, msg)

	r := resty.New().R()
	if strings.HasPrefix(notify, "http") {
		_, _ = r.
			SetQueryParams(map[string]string{
				"text": msg,
			}).
			Get(notify)
		return
	}
	token := strings.Split(notify, ":")
	switch token[0] {
	case "tg":
	case "lark":
	case "pushdeer":
		_, _ = r.
			SetQueryParams(map[string]string{
				"pushkey": token[1],
				"text":    msg,
			}).
			Post("https://api2.pushdeer.com/message/push")
	}
}
