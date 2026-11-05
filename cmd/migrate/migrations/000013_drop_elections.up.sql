-- The panel isn't a voting system: drop the election set-up tables from
-- 000009 (elections, positions, candidates, votes). The voters' master list
-- (voter_master_list) and the election switch (election_control) stay.
DROP TABLE IF EXISTS election_votes;
DROP TABLE IF EXISTS election_candidates;
DROP TABLE IF EXISTS election_positions;
DROP TABLE IF EXISTS elections;
