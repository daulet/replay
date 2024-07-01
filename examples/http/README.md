# HTTP example

This is a sample HTTP application that depends on another HTTP dependency. It meant to emulate use cases where you'd like test HTTP application without having to run the dependency service, e.g. because it is expensive to run/invoke.

* Dependency service simply returns "Hello, \<path\>" for all paths starting with "/foo" and "Hi, \<path\>" for all paths starting with "/bar". Everything else returns 404.
* Application service parses the HTTP path, squares any number it finds in path components and passes updated path to the dependency service.

Then we test the application service two distinct ways, to demonstrate how this record/replay approach could be used.

1. We write the test the standard way - define test request in code and assert expected response. Dependency service is replayed from recorded data.

2. We record incoming HTTP requests to the application service itself, hence we don't need to write test code with request and expoected responses, instead we send real requests (e.g. using curl) to a running application service and record request/response pairs. At test time, the provided test runner will enumerate recorded request/response pairs and run each as a distinct test case.
