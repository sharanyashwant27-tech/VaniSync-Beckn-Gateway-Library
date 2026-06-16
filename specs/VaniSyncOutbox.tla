---- MODULE VaniSyncOutbox ----
(*
  Formal model of the VaniSync transactional outbox for offline-first Beckn relay.

  Go refinement mapping (see docs/architecture/03-structural-view.md):
    clientDB    -> local_orders / map[string]*Order
    outbox      -> sync_queue rows with status PENDING (FIFO sequence of ids)
    network     -> in-flight relay buffer in sync.Engine
    serverDB    -> gateway-side processed order ids
    networkActive -> NetworkProbe.IsUp()

  Actions:
    LocalWrite     - atomic domain + outbox insert
    RelayOutbox    - dequeue outbox head to network when online
    NetworkDrop    - lose in-flight message; re-queue for retry
    ServerProcess  - gateway ACK; record id on server
    ToggleNetwork  - flip connectivity (models flaky Dumka links)
*)

EXTENDS Naturals, FiniteSets, Sequences, TLC

CONSTANTS OrderIds, MaxOrders

VARIABLES clientDB, outbox, network, serverDB, networkActive

vars == <<clientDB, outbox, network, serverDB, networkActive>>

\* ---------------------------------------------------------------------------
\* State predicates
\* ---------------------------------------------------------------------------

AvailableIds == OrderIds \ clientDB

TypeOK ==
  /\ clientDB \subseteq OrderIds
  /\ serverDB \subseteq clientDB
  /\ networkActive \in {TRUE, FALSE}
  /\ outbox \in Seq(OrderIds)
  /\ network \in Seq(OrderIds)
  /\ Cardinality(clientDB) <= MaxOrders
  /\ \A i \in DOMAIN outbox : outbox[i] \in clientDB
  /\ \A i \in DOMAIN network : network[i] \in clientDB
  /\ Len(outbox) + Len(network) + Cardinality(serverDB) <= Cardinality(clientDB)

Init ==
  /\ clientDB = {}
  /\ outbox = <<>>
  /\ network = <<>>
  /\ serverDB = {}
  /\ networkActive = TRUE

\* ---------------------------------------------------------------------------
\* Actions
\* ---------------------------------------------------------------------------

LocalWrite ==
  /\ AvailableIds # {}
  /\ Cardinality(clientDB) < MaxOrders
  /\ \E id \in AvailableIds :
       /\ clientDB' = clientDB \union {id}
       /\ outbox' = Append(outbox, id)
       /\ UNCHANGED <<network, serverDB, networkActive>>

RelayOutbox ==
  /\ networkActive
  /\ outbox # <<>>
  /\ LET id == Head(outbox)
     IN /\ network' = Append(network, id)
        /\ outbox' = Tail(outbox)
        /\ UNCHANGED <<clientDB, serverDB, networkActive>>

NetworkDrop ==
  /\ network # <<>>
  /\ LET id == Head(network)
     IN /\ network' = Tail(network)
        /\ outbox' = Append(outbox, id)
        /\ UNCHANGED <<clientDB, serverDB, networkActive>>

ServerProcess ==
  /\ network # <<>>
  /\ LET id == Head(network)
     IN /\ serverDB' = serverDB \union {id}
        /\ network' = Tail(network)
        /\ UNCHANGED <<clientDB, outbox, networkActive>>

ToggleNetwork ==
  /\ networkActive' = ~networkActive
  /\ UNCHANGED <<clientDB, outbox, network, serverDB>>

Next ==
  \/ LocalWrite
  \/ RelayOutbox
  \/ NetworkDrop
  \/ ServerProcess
  \/ ToggleNetwork

\* ---------------------------------------------------------------------------
\* Specification and fairness (liveness)
\* ---------------------------------------------------------------------------

FairSync ==
  /\ WF_vars(RelayOutbox)
  /\ WF_vars(ServerProcess)

Spec ==
  Init
  /\ [][Next]_vars
  /\ FairSync

\* ---------------------------------------------------------------------------
\* Properties
\* ---------------------------------------------------------------------------

\* Safety: no orphan records on the gateway — every server id was written locally.
NoOrphans == serverDB \subseteq clientDB

\* Liveness: when the network is up and work queues are drained, server matches client.
EventualConsistency ==
  [](networkActive /\ outbox = <<>> /\ network = <<>> => serverDB = clientDB)

====
