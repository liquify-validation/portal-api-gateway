# Liquify Pokt Gateway Backend

NOTE: This is still a work in progress!

Welcome to Liquify's gateway portal backend repository! This repository houses the essential components that power our gateway infrastructure, designed to seamlessly integrate with our frontend applications.

## Components

### 1. API Server
The API server is the backbone of our system, facilitating user authentication, organization management, endpoint lifecycle management, and endpoint analytics. Built with Python, it leverages a MySQL database to store all critical data.

#### Features:
- User authentication and management
- Organization creation and management
- Endpoint creation, rotation, and deletion
- Endpoint analytics for performance insights

### 2. API Gateway
The API Gateway serves as the interface between our system and the Pokt network's gateway server. Written in Lua scripting atop OpenResty (nginx), it handles inbound RPC request authentication by cross-referencing API keys with our MySQL database. And ensures the requests are routed to the target chain.

#### Key Functions:
- Routing of inbound requests to the correct chain
- Verification of API keys against stored database records
- Caching of API keys for low latency authentication

## How to Use
TODO

## Feedback and Support
If you have any questions, feedback, or require assistance, please contact our team at [contact@liquify.io](mailto:contact@liquify.io). We're here to support you in leveraging our gateway backend effectively.