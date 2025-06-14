#!/usr/bin/env node

// Minimal MCP server for testing Claude Desktop integration

const readline = require('readline');

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

// Log to stderr only
function log(message) {
  console.error(`[DEBUG] ${message}`);
}

rl.on('line', (line) => {
  try {
    const request = JSON.parse(line);
    log(`Received: ${JSON.stringify(request)}`);
    
    let response;
    
    switch (request.method) {
      case 'initialize':
        response = {
          jsonrpc: '2.0',
          id: request.id,
          result: {
            protocolVersion: '2024-11-05',
            serverInfo: {
              name: 'minimal-test-server',
              version: '0.0.1'
            },
            capabilities: {
              tools: {
                listChanged: false
              }
            }
          }
        };
        break;
        
      case 'tools/list':
        response = {
          jsonrpc: '2.0',
          id: request.id,
          result: {
            tools: [{
              name: 'test_tool',
              description: 'A simple test tool',
              inputSchema: {
                type: 'object',
                properties: {
                  message: {
                    type: 'string',
                    description: 'Test message'
                  }
                },
                required: ['message']
              }
            }]
          }
        };
        break;
        
      default:
        // Don't respond to unknown notifications
        if (request.id === undefined) {
          log(`Ignoring notification: ${request.method}`);
          return;
        }
        
        response = {
          jsonrpc: '2.0',
          id: request.id,
          error: {
            code: -32601,
            message: 'Method not found'
          }
        };
    }
    
    log(`Sending: ${JSON.stringify(response)}`);
    console.log(JSON.stringify(response));
    
  } catch (e) {
    log(`Error: ${e.message}`);
    const errorResponse = {
      jsonrpc: '2.0',
      error: {
        code: -32700,
        message: 'Parse error',
        data: e.message
      }
    };
    console.log(JSON.stringify(errorResponse));
  }
});

log('Minimal MCP server started');