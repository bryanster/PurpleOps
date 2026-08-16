from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="Session")


@_attrs_define
class Session:
    """One browser this account is signed in on, as `GET /auth/sessions`
    reports it.

    Everything here is what the server observed when the session was issued
    and last used. Nothing in it identifies the token: the value in the
    cookie is not stored — only a hash of it is — so there is no field to
    leave out by mistake.

    """

    id: UUID
    """ The session's identifier, and what `DELETE /auth/sessions/{sessionId}` takes. """
    current: bool
    """ Whether this is the session the request was made on. Exactly one row
    in a response has it, and it is the row a client marks as "this
    device" rather than offering to revoke.
     """
    created_at: datetime.datetime
    """ When this session was issued — the sign-in it belongs to. """
    last_seen_at: datetime.datetime
    """ When a request last arrived on it, to within a minute: recording
    every request would put a write in front of every read, so it is
    written back at most once a minute (`internal/authn/session`).
     """
    expires_at: datetime.datetime
    """ The absolute expiry. Nothing extends it, so a session ends at this
    moment however busy it has been. It may end sooner by going idle,
    which is a policy this document does not carry — a client that
    wanted to render "expires in" should say "expires by".
     """
    mfa_satisfied: bool
    """ Whether a second factor was presented for *this* session, as
    opposed to at some point in this account's past (M1-006). A
    deployment that has just turned the requirement on can have live
    sessions where this is false.
     """
    ip: str | Unset = UNSET
    """ The client address the session was issued to, as the server saw it
    (`internal/httpapi.clientIP`). Absent when the request did not carry
    one. It is what somebody scans the list for, and it is the server's
    observation rather than anything the client claimed.
     """
    user_agent: str | Unset = UNSET
    """ The `User-Agent` the session was issued to, absent when the request
    sent none. Attacker-controlled, like every request header: render it
    as text and never as markup.
     """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        current = self.current

        created_at = self.created_at.isoformat()

        last_seen_at = self.last_seen_at.isoformat()

        expires_at = self.expires_at.isoformat()

        mfa_satisfied = self.mfa_satisfied

        ip = self.ip

        user_agent = self.user_agent

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "current": current,
                "createdAt": created_at,
                "lastSeenAt": last_seen_at,
                "expiresAt": expires_at,
                "mfaSatisfied": mfa_satisfied,
            }
        )
        if ip is not UNSET:
            field_dict["ip"] = ip
        if user_agent is not UNSET:
            field_dict["userAgent"] = user_agent

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        current = d.pop("current")

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        last_seen_at = datetime.datetime.fromisoformat(d.pop("lastSeenAt"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expiresAt"))

        mfa_satisfied = d.pop("mfaSatisfied")

        ip = d.pop("ip", UNSET)

        user_agent = d.pop("userAgent", UNSET)

        session = cls(
            id=id,
            current=current,
            created_at=created_at,
            last_seen_at=last_seen_at,
            expires_at=expires_at,
            mfa_satisfied=mfa_satisfied,
            ip=ip,
            user_agent=user_agent,
        )

        session.additional_properties = d
        return session

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
