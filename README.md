# core-queue
Queues in go is not as prominent as in some other languages, since go excels
at handling concurrency. However, the deferrableDecorator queue can still offer some benefit
missing from the native mechanism, say go channels. The queued job won't be
lost even if the system shutdown. In other words, it means jobs can be retried
until success. Plus, it is also possible to queue the execution of a
particular job until a lengthy period of time. Useful when you need to
implement "send email after 30 days" type of Job handler.
