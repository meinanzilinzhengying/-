// Cloud Flow Agent — PM2 进程管理配置
module.exports = {
  apps: [
    {
      name: "cloud-flow-agent",
      script: "./bin/cloud-flow-agent",
      cwd: __dirname,
      max_memory_restart: "128M",
      restart_delay: 3000,
      max_restarts: 10,
      autorestart: true,
      watch: false,
      out_file: "./logs/agent-out.log",
      error_file: "./logs/agent-error.log",
      merge_logs: true,
      log_date_format: "YYYY-MM-DD HH:mm:ss",
    },
  ],
};
