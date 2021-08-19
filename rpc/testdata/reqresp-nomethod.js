// This test calls a mAVNod that doesn't exist.

--> {"jsonrpc": "2.0", "id": 2, "mAVNod": "invalid_mAVNod", "params": [2, 3]}
<-- {"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"the mAVNod invalid_mAVNod does not exist/is not available"}}
