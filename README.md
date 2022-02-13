# uptime

This repository contains two uptime-related tasks.

## GitHub Action

A simple uptime monitor for my home setup. This is a recurring GitHub Action that runs `curl` to test various sites that I have published, and sends a notification to a webhook upon a failed request.

Nothing special! Better to stay simple than overengineer this one.

## Daily Script

A daily Go task that archives folders on my home setup to S3 on a regular basis. This is distributed into my Nomad cluster as a sysbatch operation.
