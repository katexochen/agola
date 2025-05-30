// Copyright 2019 Sorint.lab
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied
// See the License for the specific language governing permissions and
// limitations under the License.

package action

import (
	"context"

	"github.com/sorintlab/errors"

	"agola.io/agola/internal/services/configstore/db"
	serrors "agola.io/agola/internal/services/errors"
	"agola.io/agola/internal/sqlg/sql"
	"agola.io/agola/internal/util"
	csapitypes "agola.io/agola/services/configstore/api/types"
	"agola.io/agola/services/configstore/types"
)

type OrgMember struct {
	User *types.User
	Role types.MemberRole
}

func orgMemberResponse(orgUser *db.OrgUser) *OrgMember {
	return &OrgMember{
		User: orgUser.User,
		Role: orgUser.Role,
	}
}

type GetOrgMembersRequest struct {
	OrgRef        string
	StartUserName string

	Limit         int
	SortDirection types.SortDirection
}

type GetOrgMembersResponse struct {
	OrgMembers []*OrgMember

	HasMore bool
}

func (h *ActionHandler) GetOrgMembers(ctx context.Context, req *GetOrgMembersRequest) (*GetOrgMembersResponse, error) {
	limit := req.Limit
	if limit > 0 {
		limit += 1
	}
	if req.SortDirection == "" {
		req.SortDirection = types.SortDirectionAsc
	}

	var dbOrgMembers []*db.OrgUser
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		var err error
		org, err := h.GetOrgByRef(tx, req.OrgRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("org %q doesn't exist", req.OrgRef), serrors.OrganizationDoesNotExist())
		}

		dbOrgMembers, err = h.d.GetOrgMembers(tx, org.ID, req.StartUserName, limit, req.SortDirection)
		return errors.WithStack(err)
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	orgMembers := make([]*OrgMember, len(dbOrgMembers))
	for i, orgUser := range dbOrgMembers {
		orgMembers[i] = orgMemberResponse(orgUser)
	}

	var hasMore bool
	if req.Limit > 0 {
		hasMore = len(orgMembers) > req.Limit
		if hasMore {
			orgMembers = orgMembers[0:req.Limit]
		}
	}

	return &GetOrgMembersResponse{
		OrgMembers: orgMembers,
		HasMore:    hasMore,
	}, nil
}

func (h *ActionHandler) GetOrg(ctx context.Context, orgRef string) (*types.Organization, error) {
	var org *types.Organization
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		var err error
		org, err = h.GetOrgByRef(tx, orgRef)
		return errors.WithStack(err)
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if org == nil {
		return nil, util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("org %q doesn't exist", orgRef), serrors.OrganizationDoesNotExist())
	}

	return org, nil
}

type GetOrgsRequest struct {
	StartOrgName string
	Visibilities []types.Visibility

	Limit         int
	SortDirection types.SortDirection
}

type GetOrgsResponse struct {
	Orgs []*types.Organization

	HasMore bool
}

func (h *ActionHandler) GetOrgs(ctx context.Context, req *GetOrgsRequest) (*GetOrgsResponse, error) {
	limit := req.Limit
	if limit > 0 {
		limit += 1
	}
	if req.SortDirection == "" {
		req.SortDirection = types.SortDirectionAsc
	}

	visibilities := req.Visibilities
	if len(visibilities) == 0 {
		// default to only public visibility
		visibilities = []types.Visibility{types.VisibilityPublic}
	}

	var orgs []*types.Organization
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		var err error
		orgs, err = h.d.GetOrgs(tx, req.StartOrgName, visibilities, limit, req.SortDirection)
		return errors.WithStack(err)
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var hasMore bool
	if req.Limit > 0 {
		hasMore = len(orgs) > req.Limit
		if hasMore {
			orgs = orgs[0:req.Limit]
		}
	}

	return &GetOrgsResponse{
		Orgs:    orgs,
		HasMore: hasMore,
	}, nil
}

type CreateOrgRequest struct {
	Name          string
	Visibility    types.Visibility
	CreatorUserID string
}

func (h *ActionHandler) CreateOrg(ctx context.Context, req *CreateOrgRequest) (*types.Organization, error) {
	if req.Name == "" {
		return nil, util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsg("organization name required"), serrors.InvalidOrganizationName())
	}
	if !util.ValidateName(req.Name) {
		return nil, util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsgf("invalid organization name %q", req.Name), serrors.InvalidOrganizationName())
	}
	if !types.IsValidVisibility(req.Visibility) {
		return nil, util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsg("invalid organization visibility"), serrors.InvalidVisibility())
	}

	var org *types.Organization
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		var err error
		// check duplicate org name
		o, err := h.d.GetOrgByName(tx, req.Name)
		if err != nil {
			return errors.WithStack(err)
		}
		if o != nil {
			return util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsgf("org %q already exists", o.Name), serrors.OrganizationAlreadyExists())
		}

		if req.CreatorUserID != "" {
			user, err := h.GetUserByRef(tx, req.CreatorUserID)
			if err != nil {
				return errors.WithStack(err)
			}
			if user == nil {
				return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("creator user %q doesn't exist", req.CreatorUserID), serrors.CreatorUserDoesNotExist())
			}
		}

		org = types.NewOrganization(tx)
		org.Name = req.Name
		org.Visibility = req.Visibility
		org.CreatorUserID = req.CreatorUserID

		if err := h.d.InsertOrganization(tx, org); err != nil {
			return errors.WithStack(err)
		}

		if org.CreatorUserID != "" {
			// add the creator as org member with role owner
			orgmember := types.NewOrganizationMember(tx)
			orgmember.OrganizationID = org.ID
			orgmember.UserID = org.CreatorUserID
			orgmember.MemberRole = types.MemberRoleOwner

			if err := h.d.InsertOrganizationMember(tx, orgmember); err != nil {
				return errors.WithStack(err)
			}
		}

		// create root org project group
		pg := types.NewProjectGroup(tx)
		// use same org visibility
		pg.Visibility = org.Visibility
		pg.Parent = types.Parent{
			Kind: types.ObjectKindOrg,
			ID:   org.ID,
		}

		if err := h.d.InsertProjectGroup(tx, pg); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return org, errors.WithStack(err)
}

type UpdateOrgRequest struct {
	Visibility types.Visibility
}

func (h *ActionHandler) UpdateOrg(ctx context.Context, orgRef string, req *UpdateOrgRequest) (*types.Organization, error) {
	if !types.IsValidVisibility(req.Visibility) {
		return nil, util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsg("invalid organization visibility"), serrors.InvalidVisibility())
	}

	var org *types.Organization
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		var err error
		// check org existance
		org, err = h.GetOrgByRef(tx, orgRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("org %q not exists", orgRef), serrors.OrganizationDoesNotExist())
		}

		org.Visibility = req.Visibility

		if err := h.d.UpdateOrganization(tx, org); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return org, errors.WithStack(err)
}

func (h *ActionHandler) DeleteOrg(ctx context.Context, orgRef string) error {
	var org *types.Organization

	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		var err error
		// check org existance
		org, err = h.GetOrgByRef(tx, orgRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("org %q doesn't exist", orgRef), serrors.OrganizationDoesNotExist())
		}

		if err := h.d.DeleteOrgMembersByOrgID(tx, org.ID); err != nil {
			return errors.WithStack(err)
		}

		if err := h.d.DeleteOrgInvitationsByOrgID(tx, org.ID); err != nil {
			return errors.WithStack(err)
		}

		// delete all projects and groups
		subgroups, err := h.getAllProjectGroupSubgroups(tx, "org/"+org.Name)
		if err != nil {
			return errors.WithStack(err)
		}
		for _, subgroup := range subgroups {
			projects, err := h.d.GetProjectGroupProjects(tx, subgroup.ID)
			if err != nil {
				return errors.WithStack(err)
			}

			for _, project := range projects {
				err = h.d.DeleteProject(tx, project.ID)
				if err != nil {
					return errors.WithStack(err)
				}
			}

			err = h.d.DeleteProjectGroup(tx, subgroup.ID)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		if err := h.d.DeleteOrganization(tx, org.ID); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return errors.WithStack(err)
}

// AddOrgMember add/updates an org member.
// TODO(sgotti) handle invitation when implemented
func (h *ActionHandler) AddOrgMember(ctx context.Context, orgRef, userRef string, role types.MemberRole) (*types.OrganizationMember, error) {
	if !types.IsValidMemberRole(role) {
		return nil, util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsgf("invalid role %q", role), serrors.InvalidRole())
	}

	var orgmember *types.OrganizationMember
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		// check existing org
		org, err := h.GetOrgByRef(tx, orgRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("org %q doesn't exists", orgRef), serrors.OrganizationDoesNotExist())
		}
		// check existing user
		user, err := h.GetUserByRef(tx, userRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if user == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("user %q doesn't exists", userRef), serrors.UserDoesNotExist())
		}

		// fetch org member if it already exist
		orgmember, err = h.d.GetOrgMemberByOrgUserID(tx, org.ID, user.ID)
		if err != nil {
			return errors.WithStack(err)
		}

		// update if role changed
		if orgmember != nil {
			if orgmember.MemberRole == role {
				return nil
			}
			orgmember.MemberRole = role
		} else {
			orgmember = types.NewOrganizationMember(tx)
			orgmember.OrganizationID = org.ID
			orgmember.UserID = user.ID
			orgmember.MemberRole = role
		}

		if err := h.d.InsertOrUpdateOrganizationMember(tx, orgmember); err != nil {
			return errors.WithStack(err)
		}

		//delete org user invitation if exists
		orgInvitation, err := h.d.GetOrgInvitationByOrgUserID(tx, org.ID, user.ID)
		if err != nil {
			return errors.WithStack(err)
		}
		if orgInvitation != nil {
			err = h.d.DeleteOrgInvitation(tx, orgInvitation.ID)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return orgmember, errors.WithStack(err)
}

// RemoveOrgMember removes an org member.
func (h *ActionHandler) RemoveOrgMember(ctx context.Context, orgRef, userRef string) error {
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		// check existing org
		org, err := h.GetOrgByRef(tx, orgRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("org %q doesn't exists", orgRef), serrors.OrganizationDoesNotExist())
		}
		// check existing user
		user, err := h.GetUserByRef(tx, userRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if user == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("user %q doesn't exists", userRef), serrors.UserDoesNotExist())
		}

		// check that org member exists
		orgmember, err := h.d.GetOrgMemberByOrgUserID(tx, org.ID, user.ID)
		if err != nil {
			return errors.WithStack(err)
		}
		if orgmember == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("orgmember for org %q, user %q doesn't exists", orgRef, userRef), serrors.OrgMemberDoesNotExist())
		}

		if err := h.d.DeleteOrganizationMember(tx, orgmember.ID); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return errors.WithStack(err)
}

func (h *ActionHandler) GetOrgInvitations(ctx context.Context, orgRef string) ([]*types.OrgInvitation, error) {
	var orgInvitations []*types.OrgInvitation
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		org, err := h.GetOrgByRef(tx, orgRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("org %q doesn't exist", orgRef), serrors.OrganizationDoesNotExist())
		}

		orgInvitations, err = h.d.GetOrgInvitations(tx, org.ID)
		if err != nil {
			return errors.WithStack(err)
		}

		return errors.WithStack(err)
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return orgInvitations, errors.WithStack(err)
}

func (h *ActionHandler) GetOrgInvitationByUserRef(ctx context.Context, orgRef, userRef string) (*types.OrgInvitation, error) {
	var orgInvitation *types.OrgInvitation
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		// check existing org
		org, err := h.GetOrgByRef(tx, orgRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("organization %q doesn't exist", orgRef), serrors.OrganizationDoesNotExist())
		}
		// check existing user
		user, err := h.GetUserByRef(tx, userRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if user == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("user %q doesn't exists", userRef), serrors.UserDoesNotExist())
		}

		orgInvitation, err = h.d.GetOrgInvitationByOrgUserID(tx, org.ID, user.ID)
		if err != nil {
			return errors.WithStack(err)
		}

		return errors.WithStack(err)
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if orgInvitation == nil {
		return nil, util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("invitation for org %q user %q doesn't exist", orgRef, userRef), serrors.InvitationDoesNotExist())
	}

	return orgInvitation, nil
}

type CreateOrgInvitationRequest struct {
	UserRef         string
	OrganizationRef string
	Role            types.MemberRole
}

func (h *ActionHandler) CreateOrgInvitation(ctx context.Context, req *CreateOrgInvitationRequest) (*types.OrgInvitation, error) {
	if req.OrganizationRef == "" {
		return nil, util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsg("organization ref required"), serrors.InvalidOrganizationName())
	}
	if req.UserRef == "" {
		return nil, util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsg("user ref required"), serrors.InvalidUserName())
	}
	if !types.IsValidMemberRole(req.Role) {
		return nil, util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsg("invalid role"), serrors.InvalidRole())
	}

	var orgInvitation *types.OrgInvitation
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		var err error

		org, err := h.GetOrgByRef(tx, req.OrganizationRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("organization %q doesn't exist", req.OrganizationRef), serrors.OrganizationDoesNotExist())
		}

		user, err := h.GetUserByRef(tx, req.UserRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if user == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("user %q doesn't exist", req.UserRef), serrors.UserDoesNotExist())
		}

		// check duplicate org invitation
		curOrgInvitation, err := h.d.GetOrgInvitationByOrgUserID(tx, org.ID, user.ID)
		if err != nil {
			return errors.WithStack(err)
		}
		if curOrgInvitation != nil {
			return util.NewAPIError(util.ErrBadRequest, util.WithAPIErrorMsg("invitation already exists"), serrors.InvitationAlreadyExists())
		}

		orgInvitation = types.NewOrgInvitation(tx)
		orgInvitation.UserID = user.ID
		orgInvitation.OrganizationID = org.ID
		orgInvitation.Role = req.Role

		if err := h.d.InsertOrgInvitation(tx, orgInvitation); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return orgInvitation, errors.WithStack(err)
}

func (h *ActionHandler) DeleteOrgInvitation(ctx context.Context, orgRef string, userRef string) error {
	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		org, err := h.GetOrgByRef(tx, orgRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("org %q doesn't exists", orgRef), serrors.OrganizationDoesNotExist())
		}
		// check existing user
		user, err := h.GetUserByRef(tx, userRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if user == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("user %q doesn't exists", userRef), serrors.UserDoesNotExist())
		}

		// check org invitation exists
		orgInvitation, err := h.d.GetOrgInvitationByOrgUserID(tx, org.ID, user.ID)
		if err != nil {
			return errors.WithStack(err)
		}
		if orgInvitation == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("invitation for org %q, user %q doesn't exists", orgRef, userRef), serrors.InvitationDoesNotExist())
		}

		if err := h.d.DeleteOrgInvitation(tx, orgInvitation.ID); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return errors.WithStack(err)
}

type OrgInvitationActionRequest struct {
	OrgRef  string
	UserRef string
	Action  csapitypes.OrgInvitationActionType
}

func (h *ActionHandler) OrgInvitationAction(ctx context.Context, req *OrgInvitationActionRequest) error {
	if !req.Action.IsValid() {
		return errors.Errorf("action is not valid")
	}

	err := h.d.Do(ctx, func(tx *sql.Tx) error {
		org, err := h.GetOrgByRef(tx, req.OrgRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if org == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("org %q doesn't exists", req.OrgRef), serrors.OrganizationDoesNotExist())
		}
		// check existing user
		user, err := h.GetUserByRef(tx, req.UserRef)
		if err != nil {
			return errors.WithStack(err)
		}
		if user == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("user %q doesn't exists", req.UserRef), serrors.UserDoesNotExist())
		}

		// check org invitation exists
		orgInvitation, err := h.d.GetOrgInvitationByOrgUserID(tx, org.ID, user.ID)
		if err != nil {
			return errors.WithStack(err)
		}
		if orgInvitation == nil {
			return util.NewAPIError(util.ErrNotExist, util.WithAPIErrorMsgf("invitation for org %q, user %q doesn't exists", req.OrgRef, req.UserRef), serrors.InvitationDoesNotExist())
		}

		if req.Action == csapitypes.Accept {
			orgMember := types.NewOrganizationMember(tx)
			orgMember.OrganizationID = orgInvitation.OrganizationID
			orgMember.UserID = orgInvitation.UserID
			orgMember.MemberRole = orgInvitation.Role

			err = h.d.InsertOrganizationMember(tx, orgMember)
			if err != nil {
				return errors.WithStack(err)
			}
		}
		err = h.d.DeleteOrgInvitation(tx, orgInvitation.ID)
		if err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
