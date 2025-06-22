package kuberhealthy

// TODO - implement a go routine that fetches all khcheck resources then iterates on them to find the pod with a matching UUID and check how long its been runninIg
// TODO - kill any pods that have run past their deadline and set their khcheck status as failed with a reason of timeout
// TODO - write an event when the khcheck failures occur to the namespace where the check ran
// TODO - clean up all Completed checks with more than 3 completed pods
