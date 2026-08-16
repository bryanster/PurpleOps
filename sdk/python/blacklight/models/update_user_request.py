from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.platform_role import PlatformRole
from ..models.user_status import UserStatus
from ..types import UNSET, Unset


T = TypeVar("T", bound="UpdateUserRequest")


@_attrs_define
class UpdateUserRequest:
    """Body of `PATCH /users/{userId}`. Every field is optional and an absent
    one is left alone, so two administrators editing different things do not
    overwrite each other. At least one must be present — an empty patch is a
    client bug, and answering it `200` would hide one.

    `email` is deliberately not a field: it is what a federated sign-in
    links an account by, so editing it could move somebody else's single
    sign-on onto this account.

    """

    display_name: str | Unset = UNSET
    platform_role: PlatformRole | Unset = UNSET
    """ What somebody may do to this installation: `admin` manages users, content
    and every engagement; `member` takes part in the engagements they belong
    to. What they may do *inside* one is `EngagementRole`, and the two are
    deliberately not the same vocabulary.
     """
    status: UserStatus | Unset = UNSET
    """ Whether an account can be used. Retirement is a status change and never
    a deletion: the executions, comments and findings somebody wrote keep
    their author (`M1-001`).
     """
    mfa_enforced: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        display_name = self.display_name

        platform_role: str | Unset = UNSET
        if not isinstance(self.platform_role, Unset):
            platform_role = self.platform_role.value

        status: str | Unset = UNSET
        if not isinstance(self.status, Unset):
            status = self.status.value

        mfa_enforced = self.mfa_enforced

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if display_name is not UNSET:
            field_dict["displayName"] = display_name
        if platform_role is not UNSET:
            field_dict["platformRole"] = platform_role
        if status is not UNSET:
            field_dict["status"] = status
        if mfa_enforced is not UNSET:
            field_dict["mfaEnforced"] = mfa_enforced

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        display_name = d.pop("displayName", UNSET)

        _platform_role = d.pop("platformRole", UNSET)
        platform_role: PlatformRole | Unset
        if isinstance(_platform_role, Unset):
            platform_role = UNSET
        else:
            platform_role = PlatformRole(_platform_role)

        _status = d.pop("status", UNSET)
        status: UserStatus | Unset
        if isinstance(_status, Unset):
            status = UNSET
        else:
            status = UserStatus(_status)

        mfa_enforced = d.pop("mfaEnforced", UNSET)

        update_user_request = cls(
            display_name=display_name,
            platform_role=platform_role,
            status=status,
            mfa_enforced=mfa_enforced,
        )

        return update_user_request
