module.exports = {
  apps: [{
    name: 'signerflow-crl',
    script: './signerflow-crl-service',
    instances: 1,
    exec_mode: 'fork',
    autorestart: true,
    watch: false,
    max_memory_restart: '500M',
    env_file: './.env',
    error_file: '/var/log/signerflow/crl-error.log',
    out_file: '/var/log/signerflow/crl-out.log',
    log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
    merge_logs: true,
    min_uptime: '10s',
    max_restarts: 10,
    restart_delay: 4000,
    kill_timeout: 5000,
    wait_ready: true,
    listen_timeout: 10000
  }]
};
