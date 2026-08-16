from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.create_user_request_status import CreateUserRequestStatus
from ..models.platform_role import PlatformRole
from ..types import UNSET, Unset


T = TypeVar("T", bound="CreateUserRequest")


@_attrs_define
class CreateUserRequest:
    """Body of `POST /users`. The identifier, the timestamps and the invite
    link are the server's; everything a caller chooses is here.

    """

    email: str
    """ The address, stored as typed and compared without regard to case or
    surrounding whitespace. An address another account already holds is
    `409`.
     """
    display_name: str
    platform_role: PlatformRole
    """ What somebody may do to this installation: `admin` manages users, content
    and every engagement; `member` takes part in the engagements they belong
    to. What they may do *inside* one is `EngagementRole`, and the two are
    deliberately not the same vocabulary.
     """
    password: str | Unset = UNSET
    """ A password for a local account. Omit it for an account that signs in
    through the identity provider.

    The bounds here are a backstop, not the policy: the policy lives in
    one place (internal/authn/password) and is reported as field errors
    on `password`, so there is one definition of an acceptable password
    rather than one here and one in the client.
     """
    status: CreateUserRequestStatus | Unset = UNSET
    """ The state to create the account in. Omitted, it is derived from
    `password`: `active` with one, `invited` without.

    `disabled` is not offered — creating an account that cannot be used
    is a way of describing an account that should not have been created.
    An `invited` account with a password is refused with a field error:
    an invited account has no local sign-in, so the password would be
    one nobody could ever use.
     """
    mfa_enforced: bool | Unset = UNSET
    """ Require a second factor of this person specifically. Defaults to false. """

    def to_dict(self) -> dict[str, Any]:
        email = self.email

        display_name = self.display_name

        platform_role = self.platform_role.value

        password = self.password

        status: str | Unset = UNSET
        if not isinstance(self.status, Unset):
            status = self.status.value

        mfa_enforced = self.mfa_enforced

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "email": email,
                "displayName": display_name,
                "platformRole": platform_role,
            }
        )
        if password is not UNSET:
            field_dict["password"] = password
        if status is not UNSET:
            field_dict["status"] = status
        if mfa_enforced is not UNSET:
            field_dict["mfaEnforced"] = mfa_enforced

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        email = d.pop("email")

        display_name = d.pop("displayName")

        platform_role = PlatformRole(d.pop("platformRole"))

        password = d.pop("password", UNSET)

        _status = d.pop("status", UNSET)
        status: CreateUserRequestStatus | Unset
        if isinstance(_status, Unset):
            status = UNSET
        else:
            status = CreateUserRequestStatus(_status)

        mfa_enforced = d.pop("mfaEnforced", UNSET)

        create_user_request = cls(
            email=email,
            display_name=display_name,
            platform_role=platform_role,
            password=password,
            status=status,
            mfa_enforced=mfa_enforced,
        )

        return create_user_request
