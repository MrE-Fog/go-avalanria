// This test calls a mavnod that doesn't exist.

--> {"jsonrpc": "2.0", "id": 2, "mavnod": "invalid_mavnod", "params": [2, 3]}
<-- {"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"the mavnod invalid_mavnod does not exist/is not available"}}
