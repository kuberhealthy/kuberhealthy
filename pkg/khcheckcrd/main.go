package khcheckcrd

import (
"sync"
)

// mu works around a potential race with a map inside the kubernetes
// api machinery which crashes when addKnownTypes and AddToGroupVersion are
// both executing at the same time.
var mu sync.Mutex
