package poker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/db"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
	"github.com/microcosm-cc/bluemonday"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"

	"go.uber.org/zap"
)

// Service represents a PostgreSQL implementation of thunderdome.PokerDataSvc.
type Service struct {
	DB                  *sql.DB
	Logger              *otelzap.Logger
	AESHashKey          string
	HTMLSanitizerPolicy *bluemonday.Policy
}

// CreateGame creates a new story pointing session
func (d *Service) CreateGame(ctx context.Context, FacilitatorID string, Name string, PointValuesAllowed []string, Stories []*thunderdome.Story, AutoFinishVoting bool, PointAverageRounding string, JoinCode string, FacilitatorCode string, HideVoterIdentity bool) (*thunderdome.Poker, error) {
	var pointValuesJSON, _ = json.Marshal(PointValuesAllowed)
	var encryptedJoinCode string
	var encryptedLeaderCode string

	if JoinCode != "" {
		EncryptedCode, codeErr := db.Encrypt(JoinCode, d.AESHashKey)
		if codeErr != nil {
			return nil, fmt.Errorf("create poker encrypt join_code error: %v", codeErr)
		}
		encryptedJoinCode = EncryptedCode
	}

	if FacilitatorCode != "" {
		EncryptedCode, codeErr := db.Encrypt(FacilitatorCode, d.AESHashKey)
		if codeErr != nil {
			return nil, fmt.Errorf("create poker encrypt leader_code error: %v", codeErr)
		}
		encryptedLeaderCode = EncryptedCode
	}

	var b = &thunderdome.Poker{
		Name:                 Name,
		Users:                make([]*thunderdome.PokerUser, 0),
		Stories:              make([]*thunderdome.Story, 0),
		VotingLocked:         true,
		PointValuesAllowed:   PointValuesAllowed,
		AutoFinishVoting:     AutoFinishVoting,
		PointAverageRounding: PointAverageRounding,
		HideVoterIdentity:    HideVoterIdentity,
		Facilitators:         make([]string, 0),
		JoinCode:             JoinCode,
		FacilitatorCode:      FacilitatorCode,
	}
	b.Facilitators = append(b.Facilitators, FacilitatorID)

	e := d.DB.QueryRowContext(ctx,
		`SELECT pokerid FROM thunderdome.poker_create($1, $2, $3, $4, $5, $6, $7, $8, null);`,
		FacilitatorID,
		Name,
		string(pointValuesJSON),
		AutoFinishVoting,
		PointAverageRounding,
		HideVoterIdentity,
		encryptedJoinCode,
		encryptedLeaderCode,
	).Scan(&b.Id)
	if e != nil {
		return nil, fmt.Errorf("poker create query error: %v", e)
	}

	for _, plan := range Stories {
		plan.Votes = make([]*thunderdome.Vote, 0)

		e := d.DB.QueryRowContext(ctx,
			`INSERT INTO thunderdome.poker_story (poker_id, name, type, reference_id, link, description, acceptance_criteria, position) 
					VALUES ($1, $2, $3, $4, $5, $6, $7, (
					  coalesce(
						(select max(position) from thunderdome.poker_story where poker_id = $1),
						-1
					  ) + 1
					)) RETURNING id`,
			b.Id,
			plan.Name,
			plan.Type,
			plan.ReferenceId,
			plan.Link,
			plan.Description,
			plan.AcceptanceCriteria,
		).Scan(&plan.Id)
		if e != nil {
			d.Logger.Error("insert stories error", zap.Error(e))
		}
	}

	b.Stories = Stories

	return b, nil
}

// TeamCreateGame creates a new story pointing session associated to a team
func (d *Service) TeamCreateGame(ctx context.Context, TeamID string, FacilitatorID string, Name string, PointValuesAllowed []string, Stories []*thunderdome.Story, AutoFinishVoting bool, PointAverageRounding string, JoinCode string, FacilitatorCode string, HideVoterIdentity bool) (*thunderdome.Poker, error) {
	var pointValuesJSON, _ = json.Marshal(PointValuesAllowed)
	var encryptedJoinCode string
	var encryptedLeaderCode string

	if JoinCode != "" {
		EncryptedCode, codeErr := db.Encrypt(JoinCode, d.AESHashKey)
		if codeErr != nil {
			return nil, fmt.Errorf("team create poker encrypt join_code error: %v", codeErr)
		}
		encryptedJoinCode = EncryptedCode
	}

	if FacilitatorCode != "" {
		EncryptedCode, codeErr := db.Encrypt(FacilitatorCode, d.AESHashKey)
		if codeErr != nil {
			return nil, fmt.Errorf("team create poker encrypt leader_code error: %v", codeErr)
		}
		encryptedLeaderCode = EncryptedCode
	}

	var b = &thunderdome.Poker{
		Name:                 Name,
		Users:                make([]*thunderdome.PokerUser, 0),
		Stories:              make([]*thunderdome.Story, 0),
		VotingLocked:         true,
		PointValuesAllowed:   PointValuesAllowed,
		AutoFinishVoting:     AutoFinishVoting,
		PointAverageRounding: PointAverageRounding,
		HideVoterIdentity:    HideVoterIdentity,
		Facilitators:         make([]string, 0),
		JoinCode:             JoinCode,
		FacilitatorCode:      FacilitatorCode,
		TeamID:               TeamID,
	}
	b.Facilitators = append(b.Facilitators, FacilitatorID)

	e := d.DB.QueryRowContext(ctx,
		`SELECT pokerid FROM thunderdome.poker_create($1, $2, $3, $4, $5, $6, $7, $8, $9);`,
		FacilitatorID,
		Name,
		string(pointValuesJSON),
		AutoFinishVoting,
		PointAverageRounding,
		HideVoterIdentity,
		encryptedJoinCode,
		encryptedLeaderCode,
		TeamID,
	).Scan(&b.Id)
	if e != nil {
		return nil, fmt.Errorf("team create poker query error: %v", e)
	}

	for _, plan := range Stories {
		plan.Votes = make([]*thunderdome.Vote, 0)

		e := d.DB.QueryRowContext(ctx,
			`INSERT INTO thunderdome.poker_story (poker_id, name, type, reference_id, link, description, acceptance_criteria, position) 
					VALUES ($1, $2, $3, $4, $5, $6, $7, (
					  coalesce(
						(select max(position) from thunderdome.poker_story where poker_id = $1),
						-1
					  ) + 1
					)) RETURNING id`,
			b.Id,
			plan.Name,
			plan.Type,
			plan.ReferenceId,
			plan.Link,
			plan.Description,
			plan.AcceptanceCriteria,
		).Scan(&plan.Id)
		if e != nil {
			d.Logger.Error("insert stories error", zap.Error(e))
		}
	}

	b.Stories = Stories

	return b, nil
}

// UpdateGame updates the game by ID
func (d *Service) UpdateGame(PokerID string, Name string, PointValuesAllowed []string, AutoFinishVoting bool, PointAverageRounding string, HideVoterIdentity bool, JoinCode string, FacilitatorCode string, TeamID string) error {
	var pointValuesJSON, _ = json.Marshal(PointValuesAllowed)
	var encryptedJoinCode string
	var encryptedLeaderCode string

	if JoinCode != "" {
		EncryptedCode, codeErr := db.Encrypt(JoinCode, d.AESHashKey)
		if codeErr != nil {
			return fmt.Errorf("update poker encrypt join_code error: %v", codeErr)
		}
		encryptedJoinCode = EncryptedCode
	}

	if FacilitatorCode != "" {
		EncryptedCode, codeErr := db.Encrypt(FacilitatorCode, d.AESHashKey)
		if codeErr != nil {
			return fmt.Errorf("update poker encrypt leader_code error: %v", codeErr)
		}
		encryptedLeaderCode = EncryptedCode
	}

	if _, err := d.DB.Exec(`
		UPDATE thunderdome.poker
		SET name = $2, point_values_allowed = $3, auto_finish_voting = $4, point_average_rounding = $5,
		 hide_voter_identity = $6, join_code = $7, leader_code = $8, updated_date = NOW(), team_id = NULLIF($9, '')::uuid
		WHERE id = $1`,
		PokerID, Name, string(pointValuesJSON), AutoFinishVoting, PointAverageRounding,
		HideVoterIdentity, encryptedJoinCode, encryptedLeaderCode, TeamID,
	); err != nil {
		return fmt.Errorf("update poker query error: %v", err)
	}

	return nil
}

// GetFacilitatorCode retrieve the game leader_code
func (d *Service) GetFacilitatorCode(PokerID string) (string, error) {
	var EncryptedLeaderCode string

	if err := d.DB.QueryRow(`
		SELECT COALESCE(leader_code, '') FROM thunderdome.poker
		WHERE id = $1`,
		PokerID,
	).Scan(&EncryptedLeaderCode); err != nil {
		return "", fmt.Errorf("get poker facilitator code query error: %v", err)
	}

	if EncryptedLeaderCode == "" {
		return "", fmt.Errorf("poker facilitator code not set")
	}
	DecryptedCode, codeErr := db.Decrypt(EncryptedLeaderCode, d.AESHashKey)
	if codeErr != nil {
		return "", fmt.Errorf("get poker facilitator code decrypt error: %v", codeErr)
	}

	return DecryptedCode, nil
}

// GetGame gets a game by ID
func (d *Service) GetGame(PokerID string, UserID string) (*thunderdome.Poker, error) {
	var b = &thunderdome.Poker{
		Id:                 PokerID,
		Users:              make([]*thunderdome.PokerUser, 0),
		Stories:            make([]*thunderdome.Story, 0),
		VotingLocked:       true,
		PointValuesAllowed: make([]string, 0),
		AutoFinishVoting:   true,
		Facilitators:       make([]string, 0),
	}

	// get game
	var pv string
	var facilitators string
	var JoinCode string
	var FacilitatorCode string
	e := d.DB.QueryRow(
		`
		SELECT b.id, b.name, b.voting_locked, COALESCE(b.active_story_id::text, ''), b.point_values_allowed, b.auto_finish_voting, 
		b.point_average_rounding, b.hide_voter_identity, COALESCE(b.join_code, ''), COALESCE(b.leader_code, ''),
		 COALESCE(b.team_id::text, ''), b.created_date, b.updated_date,
		CASE WHEN COUNT(bl) = 0 THEN '[]'::json ELSE array_to_json(array_agg(bl.user_id)) END AS leaders
		FROM thunderdome.poker b
		LEFT JOIN thunderdome.poker_facilitator bl ON b.id = bl.poker_id
		WHERE b.id = $1
		GROUP BY b.id`,
		PokerID,
	).Scan(
		&b.Id,
		&b.Name,
		&b.VotingLocked,
		&b.ActiveStoryID,
		&pv,
		&b.AutoFinishVoting,
		&b.PointAverageRounding,
		&b.HideVoterIdentity,
		&JoinCode,
		&FacilitatorCode,
		&b.TeamID,
		&b.CreatedDate,
		&b.UpdatedDate,
		&facilitators,
	)
	if e != nil {
		return nil, fmt.Errorf("get poker query error: %v", e)
	}

	_ = json.Unmarshal([]byte(facilitators), &b.Facilitators)
	_ = json.Unmarshal([]byte(pv), &b.PointValuesAllowed)

	isFacilitator := db.Contains(b.Facilitators, UserID)

	if JoinCode != "" {
		DecryptedCode, codeErr := db.Decrypt(JoinCode, d.AESHashKey)
		if codeErr != nil {
			return nil, fmt.Errorf("get poker decode join_code error: %v", codeErr)
		}
		b.JoinCode = DecryptedCode
	}

	if FacilitatorCode != "" && isFacilitator {
		DecryptedCode, codeErr := db.Decrypt(FacilitatorCode, d.AESHashKey)
		if codeErr != nil {
			return nil, fmt.Errorf("get poker decode leader_code error: %v", codeErr)
		}
		b.FacilitatorCode = DecryptedCode
	}

	b.Users = d.GetUsers(PokerID)
	b.Stories = d.GetStories(PokerID, UserID)

	return b, nil
}

// GetGamesByUser gets a list of games by UserID
func (d *Service) GetGamesByUser(UserID string, Limit int, Offset int) ([]*thunderdome.Poker, int, error) {
	var Count int
	var games = make([]*thunderdome.Poker, 0)

	e := d.DB.QueryRow(`
		WITH user_teams AS (
			SELECT t.id FROM thunderdome.team_user tu
			LEFT JOIN thunderdome.team t ON t.id = tu.team_id
			WHERE tu.user_id = $1
		),
		team_games AS (
			SELECT id FROM thunderdome.poker WHERE team_id IN (SELECT id FROM user_teams)
		),
		user_games AS (
			SELECT u.poker_id AS id FROM thunderdome.poker_user u
			WHERE u.user_id = $1 AND u.abandoned = false
		),
		games AS (
			SELECT id from user_games UNION ALL SELECT id FROM team_games
		)
		SELECT COUNT(*) FROM games;
	`, UserID).Scan(
		&Count,
	)
	if e != nil {
		return nil, Count, fmt.Errorf("get poker by user count query error: %v", e)
	}

	gameRows, gamesErr := d.DB.Query(`
		WITH user_teams AS (
			SELECT t.id, t.name FROM thunderdome.team_user tu
			LEFT JOIN thunderdome.team t ON t.id = tu.team_id
			WHERE tu.user_id = $1
		),
		team_games AS (
			SELECT id FROM thunderdome.poker WHERE team_id IN (SELECT id FROM user_teams)
		),
		user_games AS (
			SELECT u.poker_id AS id FROM thunderdome.poker_user u
			WHERE u.user_id = $1 AND u.abandoned = false
		),
		games AS (
			SELECT id from user_games UNION ALL SELECT id FROM team_games
		),
		stories AS (
			SELECT poker_id, points FROM thunderdome.poker_story WHERE poker_id IN (SELECT poker_id FROM games)
		),
		facilitators AS (
			SELECT poker_id, user_id FROM thunderdome.poker_facilitator WHERE poker_id IN (SELECT poker_id FROM games)
		)
		SELECT p.id, p.name, p.voting_locked, COALESCE(p.active_story_id::text, ''), p.point_values_allowed, p.auto_finish_voting,
		  p.point_average_rounding, p.created_date, p.updated_date,
		  CASE WHEN COUNT(s) = 0 THEN '[]'::json ELSE array_to_json(array_agg(row_to_json(s))) END AS stories,
		  CASE WHEN COUNT(bl) = 0 THEN '[]'::json ELSE array_to_json(array_agg(bl.user_id)) END AS facilitators,
		  min(COALESCE(t.name, '')) as team_name
		FROM thunderdome.poker p
		LEFT JOIN stories AS s ON s.poker_id = p.id
		LEFT JOIN facilitators AS bl ON bl.poker_id = p.id
		LEFT JOIN user_teams t ON t.id = p.team_id
		WHERE p.id IN (SELECT id FROM games)
		GROUP BY p.id ORDER BY p.created_date DESC
		LIMIT $2 OFFSET $3
	`, UserID, Limit, Offset)
	if gamesErr != nil {
		return nil, Count, fmt.Errorf("get poker by user query error: %v", gamesErr)
	}

	defer gameRows.Close()
	for gameRows.Next() {
		var stories string
		var pv string
		var facilitators string
		var b = &thunderdome.Poker{
			Users:              make([]*thunderdome.PokerUser, 0),
			Stories:            make([]*thunderdome.Story, 0),
			VotingLocked:       true,
			PointValuesAllowed: make([]string, 0),
			AutoFinishVoting:   true,
			Facilitators:       make([]string, 0),
		}
		if err := gameRows.Scan(
			&b.Id,
			&b.Name,
			&b.VotingLocked,
			&b.ActiveStoryID,
			&pv,
			&b.AutoFinishVoting,
			&b.PointAverageRounding,
			&b.CreatedDate,
			&b.UpdatedDate,
			&stories,
			&facilitators,
			&b.TeamName,
		); err != nil {
			d.Logger.Error("error getting poker by user", zap.Error(e))
		} else {
			_ = json.Unmarshal([]byte(stories), &b.Stories)
			_ = json.Unmarshal([]byte(pv), &b.PointValuesAllowed)
			_ = json.Unmarshal([]byte(facilitators), &b.Facilitators)

			games = append(games, b)
		}
	}

	return games, Count, nil
}

// ConfirmFacilitator confirms the user is a facilitator of the game
func (d *Service) ConfirmFacilitator(PokerID string, UserID string) error {
	var facilitatorID string
	var role string
	err := d.DB.QueryRow("SELECT type FROM thunderdome.users WHERE id = $1", UserID).Scan(&role)
	if err != nil {
		return fmt.Errorf("confirm poker facilitator get user role error: %v", err)
	}

	e := d.DB.QueryRow("SELECT user_id FROM thunderdome.poker_facilitator WHERE poker_id = $1 AND user_id = $2", PokerID, UserID).Scan(&facilitatorID)
	if e != nil && role != thunderdome.AdminUserType {
		return fmt.Errorf("confirm poker facilitator query error: %v", err)
	}

	return nil
}

// GetUserActiveStatus checks game active status of User
func (d *Service) GetUserActiveStatus(PokerID string, UserID string) error {
	var active bool

	err := d.DB.QueryRow(`
		SELECT coalesce(active, FALSE)
		FROM thunderdome.poker_user
		WHERE user_id = $2 AND poker_id = $1;`,
		PokerID,
		UserID,
	).Scan(
		&active,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("poker get user active status query error: %v", err)
	} else if err != nil && errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if active {
		return errors.New("DUPLICATE_BATTLE_USER")
	}

	return nil
}

// GetUsers retrieves the users for a given game
func (d *Service) GetUsers(PokerID string) []*thunderdome.PokerUser {
	var users = make([]*thunderdome.PokerUser, 0)
	rows, err := d.DB.Query(
		`SELECT
			u.id, u.name, u.type, u.avatar, pu.active, pu.spectator, COALESCE(u.email, ''), COALESCE(u.picture, '')
		FROM thunderdome.poker_user pu
		LEFT JOIN thunderdome.users u ON pu.user_id = u.id
		WHERE pu.poker_id = $1
		ORDER BY u.name`,
		PokerID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var w thunderdome.PokerUser
			if err := rows.Scan(&w.Id, &w.Name, &w.Type, &w.Avatar, &w.Active, &w.Spectator, &w.GravatarHash, &w.PictureURL); err != nil {
				d.Logger.Error("error getting poker users", zap.Error(err))
			} else {
				if w.GravatarHash != "" {
					w.GravatarHash = db.CreateGravatarHash(w.GravatarHash)
				} else {
					w.GravatarHash = db.CreateGravatarHash(w.Id)
				}
				users = append(users, &w)
			}
		}
	}

	return users
}

// GetActiveUsers retrieves the active users for a given game
func (d *Service) GetActiveUsers(PokerID string) []*thunderdome.PokerUser {
	var users = make([]*thunderdome.PokerUser, 0)
	rows, err := d.DB.Query(
		`SELECT
			w.id, w.name, w.type, w.avatar, bw.active, bw.spectator, COALESCE(w.email, ''), COALESCE(w.picture, '')
		FROM thunderdome.poker_user bw
		LEFT JOIN thunderdome.users w ON bw.user_id = w.id
		WHERE bw.poker_id = $1 AND bw.active = true
		ORDER BY w.name`,
		PokerID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var w thunderdome.PokerUser
			if err := rows.Scan(&w.Id, &w.Name, &w.Type, &w.Avatar, &w.Active, &w.Spectator, &w.GravatarHash, &w.PictureURL); err != nil {
				d.Logger.Error("error getting active poker users", zap.Error(err))
			} else {
				if w.GravatarHash != "" {
					w.GravatarHash = db.CreateGravatarHash(w.GravatarHash)
				} else {
					w.GravatarHash = db.CreateGravatarHash(w.Id)
				}
				users = append(users, &w)
			}
		}
	}

	return users
}

// AddUser adds a user by ID to the game by ID
func (d *Service) AddUser(PokerID string, UserID string) ([]*thunderdome.PokerUser, error) {
	if _, err := d.DB.Exec(
		`INSERT INTO thunderdome.poker_user (poker_id, user_id, active)
		VALUES ($1, $2, true)
		ON CONFLICT (poker_id, user_id) DO UPDATE SET active = true, abandoned = false`,
		PokerID,
		UserID,
	); err != nil {
		d.Logger.Error("error adding user to poker", zap.Error(err))
	}

	users := d.GetUsers(PokerID)

	return users, nil
}

// RetreatUser removes a user from the current game by ID
func (d *Service) RetreatUser(PokerID string, UserID string) []*thunderdome.PokerUser {
	if _, err := d.DB.Exec(
		`UPDATE thunderdome.poker_user SET active = false WHERE poker_id = $1 AND user_id = $2`, PokerID, UserID); err != nil {
		d.Logger.Error("error updating poker user to active false", zap.Error(err))
	}

	if _, err := d.DB.Exec(
		`UPDATE thunderdome.users SET last_active = NOW() WHERE id = $1`, UserID); err != nil {
		d.Logger.Error("error updating user last active timestamp", zap.Error(err))
	}

	users := d.GetUsers(PokerID)

	return users
}

// AbandonGame removes a user from the current game by ID and sets abandoned true
func (d *Service) AbandonGame(PokerID string, UserID string) ([]*thunderdome.PokerUser, error) {
	if _, err := d.DB.Exec(
		`UPDATE thunderdome.poker_user SET active = false, abandoned = true WHERE poker_id = $1 AND user_id = $2`, PokerID, UserID); err != nil {
		return nil, fmt.Errorf("error updating game user to abandoned: %v", err)
	}

	if _, err := d.DB.Exec(
		`UPDATE thunderdome.users SET last_active = NOW() WHERE id = $1`, UserID); err != nil {
		return nil, fmt.Errorf("error updating user last active timestamp: %v", err)
	}

	users := d.GetUsers(PokerID)

	return users, nil
}

// AddFacilitator makes a user a facilitator of the game
func (d *Service) AddFacilitator(PokerID string, UserID string) ([]string, error) {
	facilitators := make([]string, 0)

	if _, err := d.DB.Exec(
		`INSERT INTO thunderdome.poker_facilitator (poker_id, user_id) VALUES ($1, $2);`,
		PokerID, UserID); err != nil {
		return nil, fmt.Errorf("poker add facilitator query error: %v", err)
	}

	rows, facilitatorErr := d.DB.Query(`
		SELECT user_id FROM thunderdome.poker_facilitator WHERE poker_id = $1;
	`, PokerID)
	if facilitatorErr != nil {
		return facilitators, nil
	}

	defer rows.Close()
	for rows.Next() {
		var leader string
		if err := rows.Scan(
			&leader,
		); err != nil {
			d.Logger.Error("poker_facilitator query scan error", zap.Error(err))
		} else {
			facilitators = append(facilitators, leader)
		}
	}

	return facilitators, nil
}

// RemoveFacilitator removes a user from game facilitators
func (d *Service) RemoveFacilitator(PokerID string, UserID string) ([]string, error) {
	facilitators := make([]string, 0)
	facilitatorCount := 0
	err := d.DB.QueryRow(
		`SELECT count(user_id) FROM thunderdome.poker_facilitator WHERE poker_id = $1;`,
		PokerID,
	).Scan(&facilitatorCount)
	if err != nil {
		return nil, fmt.Errorf("poker remove facilitator query error: %v", err)
	}

	if facilitatorCount == 1 {
		return nil, fmt.Errorf("ONLY_FACILITATOR")
	}

	if _, err := d.DB.Exec(
		`DELETE FROM thunderdome.poker_facilitator WHERE poker_id = $1 AND user_id = $2;`,
		PokerID, UserID); err != nil {
		return nil, fmt.Errorf("poker remove facilitator query error: %v", err)
	}

	rows, facilitatorErr := d.DB.Query(`
		SELECT user_id FROM thunderdome.poker_facilitator WHERE poker_id = $1;
	`, PokerID)
	if facilitatorErr != nil {
		return facilitators, nil
	}

	defer rows.Close()
	for rows.Next() {
		var leader string
		if err := rows.Scan(
			&leader,
		); err != nil {
			d.Logger.Error("poker_facilitator query scan error", zap.Error(err))
		} else {
			facilitators = append(facilitators, leader)
		}
	}

	return facilitators, nil
}

// ToggleSpectator changes a game users spectator status
func (d *Service) ToggleSpectator(PokerID string, UserID string, Spectator bool) ([]*thunderdome.PokerUser, error) {
	if _, err := d.DB.Exec(
		`UPDATE thunderdome.poker_user SET spectator = $3 WHERE poker_id = $1 AND user_id = $2`, PokerID, UserID, Spectator); err != nil {
		return nil, fmt.Errorf("poker toggle spectator query error: %v", err)
	}

	if _, err := d.DB.Exec(
		`UPDATE thunderdome.users SET last_active = NOW() WHERE id = $1`, UserID); err != nil {
		d.Logger.Error("error updating user last active timestamp", zap.Error(err))
	}

	users := d.GetUsers(PokerID)

	return users, nil
}

// DeleteGame removes all game associations and the game itself by PokerID
func (d *Service) DeleteGame(PokerID string) error {
	if _, err := d.DB.Exec(
		`DELETE FROM thunderdome.poker WHERE id = $1;`, PokerID); err != nil {
		return fmt.Errorf("poker delete query error: %v", err)
	}

	return nil
}

// AddFacilitatorsByEmail adds additional game facilitators by email
func (d *Service) AddFacilitatorsByEmail(ctx context.Context, PokerID string, FacilitatorEmails []string) ([]string, error) {
	var facilitators string
	var newFacilitators []string

	for i, email := range FacilitatorEmails {
		FacilitatorEmails[i] = db.SanitizeEmail(email)
	}
	emails := strings.Join(FacilitatorEmails[:], ",")

	e := d.DB.QueryRowContext(ctx,
		`SELECT facilitators FROM thunderdome.poker_facilitator_add_by_email($1, $2);`,
		PokerID, emails,
	).Scan(&facilitators)
	if e != nil {
		return nil, fmt.Errorf("error adding poker facilitator by email: %v", e)
	}

	_ = json.Unmarshal([]byte(facilitators), &newFacilitators)

	return newFacilitators, nil
}

// GetGames gets a list of games
func (d *Service) GetGames(Limit int, Offset int) ([]*thunderdome.Poker, int, error) {
	var games = make([]*thunderdome.Poker, 0)
	var Count int

	e := d.DB.QueryRow(
		"SELECT COUNT(*) FROM thunderdome.poker;",
	).Scan(
		&Count,
	)
	if e != nil {
		return nil, Count, fmt.Errorf("get poker games count query error: %v", e)
	}

	rows, gamesErr := d.DB.Query(`
		SELECT b.id, b.name, b.voting_locked, b.active_story_id, b.point_values_allowed, b.auto_finish_voting, b.point_average_rounding, b.created_date, b.updated_date,
		CASE WHEN COUNT(bl) = 0 THEN '[]'::json ELSE array_to_json(array_agg(bl.user_id)) END AS leaders
		FROM thunderdome.poker b
		LEFT JOIN thunderdome.poker_facilitator bl ON b.id = bl.poker_id
		GROUP BY b.id ORDER BY b.created_date DESC
		LIMIT $1 OFFSET $2;
	`, Limit, Offset)
	if gamesErr != nil {
		return nil, Count, fmt.Errorf("get poker games query error: %v", gamesErr)
	}

	defer rows.Close()
	for rows.Next() {
		var pv string
		var facilitators string
		var ActiveStoryID sql.NullString
		var b = &thunderdome.Poker{
			Users:              make([]*thunderdome.PokerUser, 0),
			Stories:            make([]*thunderdome.Story, 0),
			VotingLocked:       true,
			PointValuesAllowed: make([]string, 0),
			AutoFinishVoting:   true,
			Facilitators:       make([]string, 0),
		}
		if err := rows.Scan(
			&b.Id,
			&b.Name,
			&b.VotingLocked,
			&ActiveStoryID,
			&pv,
			&b.AutoFinishVoting,
			&b.PointAverageRounding,
			&b.CreatedDate,
			&b.UpdatedDate,
			&facilitators,
		); err != nil {
			d.Logger.Error("get poker games query error", zap.Error(err))
		} else {
			_ = json.Unmarshal([]byte(pv), &b.PointValuesAllowed)
			_ = json.Unmarshal([]byte(facilitators), &b.Facilitators)
			b.ActiveStoryID = ActiveStoryID.String
			games = append(games, b)
		}
	}

	return games, Count, nil
}

// GetActiveGames gets a list of active games
func (d *Service) GetActiveGames(Limit int, Offset int) ([]*thunderdome.Poker, int, error) {
	var games = make([]*thunderdome.Poker, 0)
	var Count int

	e := d.DB.QueryRow(
		"SELECT COUNT(DISTINCT pu.poker_id) FROM thunderdome.poker_user pu WHERE pu.active IS TRUE;",
	).Scan(
		&Count,
	)
	if e != nil {
		return nil, Count, fmt.Errorf("get active poker games count query error: %v", e)
	}

	rows, gamesErr := d.DB.Query(`
		SELECT b.id, b.name, b.voting_locked, b.active_story_id, b.point_values_allowed, b.auto_finish_voting, b.point_average_rounding, b.created_date, b.updated_date,
		CASE WHEN COUNT(bl) = 0 THEN '[]'::json ELSE array_to_json(array_agg(bl.user_id)) END AS leaders
		FROM thunderdome.poker_user bu
		LEFT JOIN thunderdome.poker b ON b.id = bu.poker_id
		LEFT JOIN thunderdome.poker_facilitator bl ON b.id = bl.poker_id
		WHERE bu.active IS TRUE GROUP BY b.id
		LIMIT $1 OFFSET $2;
	`, Limit, Offset)
	if gamesErr != nil {
		return nil, Count, fmt.Errorf("get active poker games query error: %v", gamesErr)
	}

	defer rows.Close()
	for rows.Next() {
		var pv string
		var facilitators string
		var ActiveStoryID sql.NullString
		var b = &thunderdome.Poker{
			Users:              make([]*thunderdome.PokerUser, 0),
			Stories:            make([]*thunderdome.Story, 0),
			VotingLocked:       true,
			PointValuesAllowed: make([]string, 0),
			AutoFinishVoting:   true,
			Facilitators:       make([]string, 0),
		}
		if err := rows.Scan(
			&b.Id,
			&b.Name,
			&b.VotingLocked,
			&ActiveStoryID,
			&pv,
			&b.AutoFinishVoting,
			&b.PointAverageRounding,
			&b.CreatedDate,
			&b.UpdatedDate,
			&facilitators,
		); err != nil {
			d.Logger.Error("get active poker games query error", zap.Error(err))
		} else {
			_ = json.Unmarshal([]byte(pv), &b.PointValuesAllowed)
			_ = json.Unmarshal([]byte(facilitators), &b.Facilitators)
			b.ActiveStoryID = ActiveStoryID.String
			games = append(games, b)
		}
	}

	return games, Count, nil
}

// PurgeOldGames deletes games older than {DaysOld} days
func (d *Service) PurgeOldGames(ctx context.Context, DaysOld int) error {
	if _, err := d.DB.ExecContext(ctx,
		`DELETE FROM thunderdome.poker WHERE last_active < (NOW() - $1 * interval '1 day');`,
		DaysOld,
	); err != nil {
		return fmt.Errorf("clean poker games query error: %v", err)
	}

	return nil
}
