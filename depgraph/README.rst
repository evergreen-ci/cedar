================================================
``depgraph`` -- MongoDB Library Dependency Graph
================================================

The MongoDB build system produces "library dependency" data, which
can provide insight into the structure of the core MongoDB
components. You can download this artifact in the ``compile_all`` task
of the `shared library builder in evergreen
<https://evergreen.mongodb.com/waterfall/mongodb-mongo-master>`_.

``depgraph`` is a go library for this data structure for use in other
projects that wish to parse this format.
