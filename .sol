id = "pularis"
name = "Pularis"
version = "0.1.0"
enabled = true
description = "Generic external API poller and Redis event publisher"
author = ""

[[workers]]
id = "main"
enabled = false
workdir = "core"
command = ["go", "run", "."]
env = { REDIS_ADDR = "${REDIS_ADDR}", CONFIGS_DIR = "${CONFIGS_DIR}" }
