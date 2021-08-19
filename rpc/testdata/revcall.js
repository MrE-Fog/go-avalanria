// This test checks reverse calls.

--> {"jsonrpc":"2.0","id":2,"mavnod":"test_callMeBack","params":["foo",[1]]}
<-- {"jsonrpc":"2.0","id":1,"mavnod":"foo","params":[1]}
--> {"jsonrpc":"2.0","id":1,"result":"my result"}
<-- {"jsonrpc":"2.0","id":2,"result":"my result"}
