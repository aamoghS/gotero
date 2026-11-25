Gotero is an open-source Go client for interacting with the Zotero API . It enables users to fetch Zotero collections and download PDFs concurrently, making it easy to automate and manage research libraries programmatically.

## System design

How a request or a run moves through the code, then the choices behind that shape. In the decision charts, a **solid** box is the route this repo takes and a **dashed** box is the alternative it turns down; the edge label is the tradeoff.

### A collection of PDFs

The CLI takes a collection, including subcollections, and an output directory. Workers download in parallel. Each file is one request. Zotero stays the source of truth.

```mermaid
flowchart TD
  classDef chosen fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#1b5e20
  key["Zotero API key"]
  col["Collection and subcollections"]
  workers["Concurrent workers"]
  disk["PDFs on disk"]
  key --> col --> workers --> disk
  class key,col,workers,disk chosen
```

### Parallel, and no second index

One-at-a-time spends the time on round trips. The run this was pointed at pulled 5,000-plus documents and raised throughput 65% against that. Gotero does not keep its own index, and it does not delete a file Zotero removed.

```mermaid
flowchart TD
  classDef chosen fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#1b5e20
  classDef rejected fill:#f5f5f5,stroke:#9e9e9e,stroke-width:1px,stroke-dasharray:5 5,color:#616161
  q{How the files are fetched}
  q -->|"one file, one request, shared only at the folder"| par["Parallel workers"]
  q -->|"the next file waits"| seq["One at a time"]
  index{A local catalog}
  index -->|"Zotero is the list"| api["No second index"]
  index -->|"also delete what Zotero deleted"| mirror["A mirror"]
  class par,api chosen
  class seq,mirror rejected
```

Longer write-up: [aamogh.vercel.app/projects/gotero](https://aamogh.vercel.app/projects/gotero.html)
