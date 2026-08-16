from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.sso_provider_id import SSOProviderId


T = TypeVar("T", bound="SSOProvider")


@_attrs_define
class SSOProvider:
    """One single sign-on button."""

    id: SSOProviderId
    """ Which protocol this provider speaks. """
    label: str
    """ What to write on the button. It names the protocol rather than the
    provider: the issuer URL is configuration, and putting it on a public
    page would tell an unauthenticated caller which directory this
    organization uses.
     """
    start_url: str
    """ Where to send the browser to begin. A path within this deployment,
    relative to the API base — navigate to it, do not fetch it.
     """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id.value

        label = self.label

        start_url = self.start_url

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "label": label,
                "startUrl": start_url,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = SSOProviderId(d.pop("id"))

        label = d.pop("label")

        start_url = d.pop("startUrl")

        sso_provider = cls(
            id=id,
            label=label,
            start_url=start_url,
        )

        sso_provider.additional_properties = d
        return sso_provider

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
