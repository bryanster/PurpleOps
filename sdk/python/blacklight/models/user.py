from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.platform_role import PlatformRole
from ..models.user_status import UserStatus
from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="User")


@_attrs_define
class User:
    """One account, as user administration reports it. Compare `CurrentUser`,
    which is the caller describing themselves and carries their memberships
    and their MFA state; this is an administrator describing somebody else.

    What is *not* here is the point of it: there is no password hash, no
    authenticator secret, no recovery code and no session token, so no
    response built from this schema can carry one.

    """

    id: UUID
    email: str
    """ The address as it was typed. Matched without regard to case everywhere it is looked up. """
    display_name: str
    platform_role: PlatformRole
    """ What somebody may do to this installation: `admin` manages users, content
    and every engagement; `member` takes part in the engagements they belong
    to. What they may do *inside* one is `EngagementRole`, and the two are
    deliberately not the same vocabulary.
     """
    status: UserStatus
    """ Whether an account can be used. Retirement is a status change and never
    a deletion: the executions, comments and findings somebody wrote keep
    their author (`M1-001`).
     """
    mfa_enforced: bool
    """ Whether an administrator requires a second factor of this person
    specifically. It is one input to the effective requirement and not
    the whole answer — see `MFAState.required`, which is what an
    interface acts on for the *signed-in* user.
     """
    created_at: datetime.datetime
    updated_at: datetime.datetime
    last_login_at: datetime.datetime | Unset = UNSET
    """ When this account last signed in. Absent if it never has. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        email = self.email

        display_name = self.display_name

        platform_role = self.platform_role.value

        status = self.status.value

        mfa_enforced = self.mfa_enforced

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        last_login_at: str | Unset = UNSET
        if not isinstance(self.last_login_at, Unset):
            last_login_at = self.last_login_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "email": email,
                "displayName": display_name,
                "platformRole": platform_role,
                "status": status,
                "mfaEnforced": mfa_enforced,
                "createdAt": created_at,
                "updatedAt": updated_at,
            }
        )
        if last_login_at is not UNSET:
            field_dict["lastLoginAt"] = last_login_at

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        email = d.pop("email")

        display_name = d.pop("displayName")

        platform_role = PlatformRole(d.pop("platformRole"))

        status = UserStatus(d.pop("status"))

        mfa_enforced = d.pop("mfaEnforced")

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updatedAt"))

        _last_login_at = d.pop("lastLoginAt", UNSET)
        last_login_at: datetime.datetime | Unset
        if isinstance(_last_login_at, Unset):
            last_login_at = UNSET
        else:
            last_login_at = datetime.datetime.fromisoformat(_last_login_at)

        user = cls(
            id=id,
            email=email,
            display_name=display_name,
            platform_role=platform_role,
            status=status,
            mfa_enforced=mfa_enforced,
            created_at=created_at,
            updated_at=updated_at,
            last_login_at=last_login_at,
        )

        user.additional_properties = d
        return user

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
