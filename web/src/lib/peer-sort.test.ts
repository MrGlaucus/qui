import { describe, expect, it } from "vitest"
import type { SortedPeer } from "@/types"
import { sortPeers } from "./peer-sort"

describe("sortPeers", () => {
  const peers: SortedPeer[] = [
    { key: "10.0.0.10:80", ip: "10.0.0.10", progress: 0.2, up_speed: 0, dl_speed: 300, uploaded: 20, downloaded: 50, client: "Client 10" },
    { key: "10.0.0.2:80", ip: "10.0.0.2", progress: 0.9, up_speed: 200, dl_speed: 100, uploaded: 50, downloaded: 20, client: "Client 2" },
    { key: "10.0.0.3:80", ip: "10.0.0.3", progress: 0.5 },
  ]

  it("sorts numbers in both directions without mutating the input", () => {
    expect(sortPeers(peers, { field: "up_speed", direction: "desc" }, {}).map(p => p.up_speed)).toEqual([200, 0, undefined])
    expect(sortPeers(peers, { field: "up_speed", direction: "asc" }, {}).map(p => p.up_speed)).toEqual([0, 200, undefined])
    expect(peers[0].up_speed).toBe(0)
  })

  it.each(["dl_speed", "uploaded", "downloaded", "progress"] as const)("sorts by %s", field => {
    const result = sortPeers(peers, { field, direction: "desc" }, {}).map(p => p[field])
    expect(result[0]).toBe(Math.max(...peers.map(p => p[field] ?? 0)))
  })

  it("sorts addresses and client names naturally", () => {
    expect(sortPeers(peers, { field: "key", direction: "asc" }, {})[0].ip).toBe("10.0.0.2")
    expect(sortPeers(peers, { field: "client", direction: "asc" }, {})[0].client).toBe("Client 2")
  })

  it("keeps unresolved ISPs last and uses updated lookup results", () => {
    const data = { "10.0.0.10": "Provider A", "10.0.0.2": "loading", "10.0.0.3": null }
    for (const direction of ["asc", "desc"] as const) {
      expect(sortPeers(peers, { field: "isp", direction }, data)[0].ip).toBe("10.0.0.10")
    }
    data["10.0.0.2"] = "Provider Z"
    expect(sortPeers(peers, { field: "isp", direction: "desc" }, data)[0].ip).toBe("10.0.0.2")
  })
})
