---
title: "Queue Assistant"
summary: "A Chrome and Edge extension that watches an IT analyst's ServiceNow ticket queue and sends reminders and digests to Slack."
status: "shipped"           # [confirm: shipped, or still in progress?]
stack: ["JavaScript", "Python", "Chrome Extensions", "ServiceNow", "Slack Webhooks"]
repo: ""                    # [confirm: GitHub repo link, or leave empty if it's a work project]
demo: ""
date: 2025-01-01            # [confirm: start date]
ended: ""
featured: true              # [confirm: feature it on the page?]
posts: []
draft: false
---

## What it does

A browser extension for Chrome and Edge that helps an IT analyst stay on top of their ServiceNow ticket queue.

- Periodically sends GET requests to ServiceNow to pull the analyst's current queue.
- Lets the analyst set reminder times and open tickets right from the extension window.
- Sends digests and reminders to the analyst's Slack account through Slack webhooks.

## How it's built

The extension is written in JavaScript, with a few Python scripts to help customize the app.
It doesn't need an OAuth integration with ServiceNow or Slack.

<!-- confirm: why you built it, and any result you want to mention, such as tickets handled or time saved -->
